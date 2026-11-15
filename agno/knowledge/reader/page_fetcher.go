package reader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Page fetching for URL readers. Port of agno-python
// knowledge/reader/page_fetcher.py: a PageFetcher turns a list of page URLs
// into whole-page text, recording which extractor produced each page and what
// was tried. HTTPPageFetcher is the built-in fetcher (Python's
// HttpxPageFetcher). Python's ParallelPageFetcher (Parallel's extraction API
// with this fetcher as fallback) is not ported; plug any provider in by
// implementing PageFetcher.

// FetchAttempt records one extractor attempt for a page.
type FetchAttempt struct {
	Extractor string `json:"extractor"`
	Outcome   string `json:"outcome"`
}

// FetchedPage is one page's fetch result: the whole page as text, unchunked,
// with provenance.
type FetchedPage struct {
	URL       string
	Content   string
	Title     string
	Extractor string
	Error     string
	Attempts  []FetchAttempt
}

// OK reports whether the page was fetched and has content.
func (p FetchedPage) OK() bool { return p.Error == "" && p.Content != "" }

// PageFetcher turns page URLs into FetchedPage results, in input order,
// never failing per page (errors are recorded on the page).
type PageFetcher interface {
	FetchMany(ctx context.Context, urls []string, allowedHosts []string) []FetchedPage
}

// HTTPPageFetcher fetches pages over HTTP and extracts main-content text from
// HTML (text/plain and text/markdown bodies are kept as-is, PDFs go through
// PDFReader). Fetching a list the caller already chose is not crawling, so
// there is no politeness delay; Concurrency bounds requests in flight.
type HTTPPageFetcher struct {
	NetOptions
	// Concurrency bounds parallel requests (default 8).
	Concurrency int
}

// NewHTTPPageFetcher creates the built-in page fetcher.
func NewHTTPPageFetcher() *HTTPPageFetcher {
	return &HTTPPageFetcher{Concurrency: 8}
}

const httpExtractorID = "http"

// FetchMany implements PageFetcher. allowedHosts (when non-nil) is enforced
// in addition to NetOptions.AllowedHosts, on the URL and every redirect hop.
func (f *HTTPPageFetcher) FetchMany(ctx context.Context, urls []string, allowedHosts []string) []FetchedPage {
	opts := f.NetOptions
	if allowedHosts != nil {
		opts.AllowedHosts = intersectHosts(opts.AllowedHosts, allowedHosts)
	}
	n := f.Concurrency
	if n <= 0 {
		n = 8
	}
	out := make([]FetchedPage, len(urls))
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			page := f.fetchOne(ctx, &opts, u)
			outcome := page.Error
			if outcome == "" {
				outcome = "ok"
			}
			page.Attempts = append(page.Attempts, FetchAttempt{Extractor: httpExtractorID, Outcome: outcome})
			out[i] = page
		}(i, u)
	}
	wg.Wait()
	return out
}

func intersectHosts(a, b []string) []string {
	if a == nil {
		return b
	}
	set := map[string]bool{}
	for _, h := range a {
		set[strings.ToLower(h)] = true
	}
	out := []string{}
	for _, h := range b {
		if set[strings.ToLower(h)] {
			out = append(out, h)
		}
	}
	return out
}

func (f *HTTPPageFetcher) fetchOne(ctx context.Context, opts *NetOptions, u string) FetchedPage {
	if !IsHostAllowed(u, opts.AllowedHosts) {
		return FetchedPage{URL: u, Error: "off-host", Extractor: httpExtractorID}
	}
	res, err := opts.get(ctx, u, nil)
	if err != nil {
		return FetchedPage{URL: u, Error: failureReason(err), Extractor: httpExtractorID}
	}
	return pageFromResponse(ctx, u, res)
}

func failureReason(err error) string {
	var se *HTTPStatusError
	if errors.As(err, &se) {
		return fmt.Sprintf("HTTP %d", se.StatusCode)
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Client.Timeout") {
		return "timeout"
	}
	if errors.Is(err, ErrHostNotAllowed) {
		return "off-host"
	}
	msg := err.Error()
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
}

func pageFromResponse(ctx context.Context, u string, res *fetchResult) FetchedPage {
	ct := mediaType(res.ContentType)
	var page FetchedPage
	if ct == "application/pdf" || bytes.HasPrefix(res.Body, []byte("%PDF-")) {
		docs, err := NewPDFReader().ReadBytes(ctx, res.Body, u)
		if err != nil {
			return FetchedPage{URL: u, Error: "pdf: " + err.Error(), Extractor: httpExtractorID}
		}
		var parts []string
		for _, d := range docs {
			if d.Content != "" {
				parts = append(parts, d.Content)
			}
		}
		page = FetchedPage{URL: u, Content: strings.Join(parts, "\n\n"), Extractor: httpExtractorID}
	} else {
		body := string(res.Body)
		switch {
		case ct == "text/plain" || ct == "text/markdown":
			page = FetchedPage{URL: u, Content: body, Extractor: httpExtractorID}
		case ct == "text/html" || looksLikeHTML(body):
			page = FetchedPage{URL: u, Content: ExtractPageText(body), Title: ExtractPageTitle(body), Extractor: httpExtractorID}
		default:
			if ct == "" {
				ct = "unknown content type"
			}
			return FetchedPage{URL: u, Error: "not-html (" + ct + ")", Extractor: httpExtractorID}
		}
	}
	if page.Content == "" {
		return FetchedPage{URL: u, Title: page.Title, Error: "empty", Extractor: httpExtractorID}
	}
	return page
}

var _ PageFetcher = (*HTTPPageFetcher)(nil)

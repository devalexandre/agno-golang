package reader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/devalexandre/agno-golang/agno/document"
)

// WebSearchResult is one search hit.
type WebSearchResult struct {
	Title       string
	URL         string
	Description string
}

// WebSearcher runs a web search.
type WebSearcher interface {
	Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error)
}

// DuckDuckGoSearcher searches DuckDuckGo's keyless HTML endpoint
// (html.duckduckgo.com), mirroring Python's use of the ddgs package.
type DuckDuckGoSearcher struct {
	// BaseURL (default https://html.duckduckgo.com/html/); overridable for tests.
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	// MaxRetries for rate-limited/failed searches (default 2).
	MaxRetries int
	// RetryDelay before a retry (default 5s, doubled each attempt).
	RetryDelay time.Duration
}

// Search implements WebSearcher.
func (s *DuckDuckGoSearcher) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	base := s.BaseURL
	if base == "" {
		base = "https://html.duckduckgo.com/html/"
	}
	client := apiClient(s.HTTPClient, 15*time.Second)
	retries := s.MaxRetries
	if retries <= 0 {
		retries = 2
	}
	delay := s.RetryDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay * time.Duration(1<<(attempt-1))):
			}
		}
		results, err := s.search(ctx, client, base, query, maxResults)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (s *DuckDuckGoSearcher) search(ctx context.Context, client *http.Client, base, query string, maxResults int) ([]WebSearchResult, error) {
	form := url.Values{"q": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ua := s.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo: HTTP %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	var out []WebSearchResult
	doc.Find(".result").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		if sel.HasClass("result--ad") {
			return true
		}
		a := sel.Find("a.result__a").First()
		href, _ := a.Attr("href")
		href = ddgTargetURL(href)
		if href == "" {
			return true
		}
		out = append(out, WebSearchResult{
			Title:       strings.TrimSpace(a.Text()),
			URL:         href,
			Description: strings.TrimSpace(sel.Find(".result__snippet").First().Text()),
		})
		return maxResults <= 0 || len(out) < maxResults
	})
	return out, nil
}

// ddgTargetURL unwraps DuckDuckGo's /l/?uddg= redirect links.
func ddgTargetURL(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if strings.HasSuffix(u.Hostname(), "duckduckgo.com") && strings.HasPrefix(u.Path, "/l/") {
		if t := u.Query().Get("uddg"); t != "" {
			return t
		}
	}
	return href
}

// WebSearchReader searches the web for a query and reads the result pages.
// Port of agno-python knowledge/reader/web_search_reader.py (DuckDuckGo by
// default; any WebSearcher can be plugged in). Each fetched page becomes a
// document with url/search_title/search_description metadata.
type WebSearchReader struct {
	BaseReader
	NetOptions
	Searcher WebSearcher
	// MaxResults bounds the documents returned (default 5).
	MaxResults int
	// DelayBetweenRequests between page fetches (default 2s, as in Python).
	DelayBetweenRequests time.Duration
	// MaxRetries per page fetch (default 3).
	MaxRetries int
	// RetryDelay between page fetch retries (default 1s).
	RetryDelay time.Duration
	// SearchEngine label recorded in metadata (default "duckduckgo").
	SearchEngine string
}

// NewWebSearchReader creates a DuckDuckGo-backed web search reader.
func NewWebSearchReader() *WebSearchReader {
	return &WebSearchReader{
		BaseReader:           BaseReader{ID: "web_search"},
		Searcher:             &DuckDuckGoSearcher{},
		MaxResults:           5,
		DelayBetweenRequests: 2 * time.Second,
		MaxRetries:           3,
		RetryDelay:           time.Second,
		SearchEngine:         "duckduckgo",
	}
}

// WithPublicOnly enables the SSRF-safe client for result pages.
func (r *WebSearchReader) WithPublicOnly(enabled bool) *WebSearchReader {
	r.PublicOnly = enabled
	return r
}

// extractSearchPageText mirrors Python's _extract_text_from_html: drop
// script/style and collapse the page text onto one line.
func extractSearchPageText(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}
	doc.Find("script, style").Remove()
	var parts []string
	for _, line := range strings.Split(doc.Text(), "\n") {
		for _, phrase := range strings.Split(strings.TrimSpace(line), "  ") {
			if p := strings.TrimSpace(phrase); p != "" {
				parts = append(parts, p)
			}
		}
	}
	return strings.Join(parts, " ")
}

func (r *WebSearchReader) fetch(ctx context.Context, u string) (string, error) {
	retries := r.MaxRetries
	if retries <= 0 {
		retries = 1
	}
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 && r.RetryDelay > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(r.RetryDelay):
			}
		}
		res, err := r.get(ctx, u, nil)
		if err != nil {
			lastErr = err
			if errors.Is(err, ErrHostNotAllowed) || errors.Is(err, ErrNonPublicAddress) || errors.Is(err, ErrNonPublicURL) {
				break
			}
			continue
		}
		if strings.Contains(strings.ToLower(res.ContentType), "text/html") {
			return extractSearchPageText(string(res.Body)), nil
		}
		return string(res.Body), nil
	}
	return "", lastErr
}

// Read searches for query (source) and returns one document per fetched result.
func (r *WebSearchReader) Read(ctx context.Context, query string) ([]*document.Document, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("web_search: query cannot be empty")
	}
	max := r.MaxResults
	if max <= 0 {
		max = 5
	}
	searcher := r.Searcher
	if searcher == nil {
		searcher = &DuckDuckGoSearcher{}
	}
	results, err := searcher.Search(ctx, query, max)
	if err != nil {
		return nil, fmt.Errorf("web_search: %w", err)
	}
	engine := r.SearchEngine
	if engine == "" {
		engine = "duckduckgo"
	}
	visited := map[string]bool{}
	var docs []*document.Document
	for _, res := range results {
		u, err := url.Parse(res.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || visited[res.URL] {
			continue
		}
		if !IsHostAllowed(res.URL, r.AllowedHosts) {
			continue
		}
		visited[res.URL] = true
		if len(docs) > 0 && r.DelayBetweenRequests > 0 {
			select {
			case <-ctx.Done():
				return docs, ctx.Err()
			case <-time.After(r.DelayBetweenRequests):
			}
		}
		content, err := r.fetch(ctx, res.URL)
		if err != nil {
			continue
		}
		d := document.NewDocument(content)
		d.ID = res.URL
		d.Name = res.Title
		if d.Name == "" {
			d.Name = u.Host
		}
		d.Source = res.URL
		d.ContentType = "text/plain"
		d.Metadata["url"] = res.URL
		d.Metadata["search_title"] = res.Title
		d.Metadata["search_description"] = res.Description
		d.Metadata["source"] = "web_search"
		d.Metadata["search_engine"] = engine
		docs = append(docs, d)
		if len(docs) >= max {
			break
		}
	}
	return docs, nil
}

var _ Reader = (*WebSearchReader)(nil)
var _ WebSearcher = (*DuckDuckGoSearcher)(nil)

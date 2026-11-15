package reader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// FirecrawlMode selects how FirecrawlReader reads a URL.
type FirecrawlMode string

const (
	// FirecrawlScrape scrapes a single page (default).
	FirecrawlScrape FirecrawlMode = "scrape"
	// FirecrawlCrawl crawls the site starting at the URL and waits for the job.
	FirecrawlCrawl FirecrawlMode = "crawl"
)

// FirecrawlReader reads web pages as markdown through the Firecrawl REST API
// (https://docs.firecrawl.dev). Port of agno-python
// knowledge/reader/firecrawl_reader.py, which wraps firecrawl-py's
// scrape_url / crawl_url; this calls POST /v1/scrape and POST /v1/crawl +
// GET /v1/crawl/{id} (polling until the job completes) directly.
type FirecrawlReader struct {
	BaseReader
	// APIKey (default: FIRECRAWL_API_KEY env var).
	APIKey string
	// Params are merged into the request body (e.g. {"onlyMainContent": true}
	// for scrape, {"limit": 10} for crawl).
	Params map[string]any
	Mode   FirecrawlMode
	// AllowedHosts, when non-nil, refuses URLs whose host is not listed.
	AllowedHosts []string
	// BaseURL of the API (default https://api.firecrawl.dev).
	BaseURL    string
	HTTPClient *http.Client
	// PollInterval between crawl status checks (default 2s).
	PollInterval time.Duration
	// CrawlTimeout bounds the wait for a crawl job (default 10 min).
	CrawlTimeout time.Duration
}

// NewFirecrawlReader creates a Firecrawl reader in scrape mode.
func NewFirecrawlReader(apiKey string) *FirecrawlReader {
	return &FirecrawlReader{
		BaseReader: BaseReader{ID: "firecrawl", Extensions: []string{"http", "https"}},
		APIKey:     apiKey,
		Mode:       FirecrawlScrape,
		BaseURL:    "https://api.firecrawl.dev",
	}
}

type firecrawlPage struct {
	Markdown string         `json:"markdown"`
	Metadata map[string]any `json:"metadata"`
}

func (r *FirecrawlReader) headers() (map[string]string, error) {
	key := r.APIKey
	if key == "" {
		key = os.Getenv("FIRECRAWL_API_KEY")
	}
	if key == "" {
		return nil, errors.New("firecrawl: API key not set (FIRECRAWL_API_KEY)")
	}
	return map[string]string{"Authorization": "Bearer " + key}, nil
}

func (r *FirecrawlReader) base() string {
	if r.BaseURL == "" {
		return "https://api.firecrawl.dev"
	}
	return strings.TrimRight(r.BaseURL, "/")
}

// Read scrapes or crawls source depending on Mode.
func (r *FirecrawlReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	if !IsHostAllowed(source, r.AllowedHosts) {
		return nil, fmt.Errorf("firecrawl: %w: %s", ErrHostNotAllowed, source)
	}
	switch r.Mode {
	case "", FirecrawlScrape:
		return r.Scrape(ctx, source)
	case FirecrawlCrawl:
		return r.Crawl(ctx, source)
	default:
		return nil, fmt.Errorf("firecrawl: mode %q not implemented", r.Mode)
	}
}

func (r *FirecrawlReader) newDoc(source, id, content string, meta map[string]any) *document.Document {
	d := document.NewDocument(content)
	d.ID = id
	d.Name = source
	d.Source = id
	d.ContentType = "text/markdown"
	d.Metadata["url"] = id
	for k, v := range meta {
		if _, exists := d.Metadata[k]; !exists {
			d.Metadata[k] = v
		}
	}
	return d
}

// Scrape reads a single page.
func (r *FirecrawlReader) Scrape(ctx context.Context, source string) ([]*document.Document, error) {
	h, err := r.headers()
	if err != nil {
		return nil, err
	}
	body := map[string]any{"url": source, "formats": []string{"markdown"}}
	for k, v := range r.Params {
		body[k] = v
	}
	var resp struct {
		Success bool          `json:"success"`
		Error   string        `json:"error"`
		Data    firecrawlPage `json:"data"`
	}
	client := apiClient(r.HTTPClient, 120*time.Second)
	if err := doJSON(ctx, client, http.MethodPost, r.base()+"/v1/scrape", h, body, &resp); err != nil {
		return nil, fmt.Errorf("firecrawl: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return nil, fmt.Errorf("firecrawl: scrape failed: %s", resp.Error)
	}
	return []*document.Document{r.newDoc(source, source, resp.Data.Markdown, resp.Data.Metadata)}, nil
}

// Crawl starts a crawl job at source and waits for its pages.
func (r *FirecrawlReader) Crawl(ctx context.Context, source string) ([]*document.Document, error) {
	h, err := r.headers()
	if err != nil {
		return nil, err
	}
	body := map[string]any{"url": source, "scrapeOptions": map[string]any{"formats": []string{"markdown"}}}
	for k, v := range r.Params {
		body[k] = v
	}
	client := apiClient(r.HTTPClient, 60*time.Second)
	var start struct {
		Success bool   `json:"success"`
		ID      string `json:"id"`
		Error   string `json:"error"`
	}
	if err := doJSON(ctx, client, http.MethodPost, r.base()+"/v1/crawl", h, body, &start); err != nil {
		return nil, fmt.Errorf("firecrawl: %w", err)
	}
	if start.ID == "" {
		return nil, fmt.Errorf("firecrawl: crawl not started: %s", start.Error)
	}
	timeout := r.CrawlTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	interval := r.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	baseURL, _ := url.Parse(r.base())
	statusURL := r.base() + "/v1/crawl/" + url.PathEscape(start.ID)
	var pages []firecrawlPage
	for {
		var st struct {
			Status string          `json:"status"`
			Data   []firecrawlPage `json:"data"`
			Next   string          `json:"next"`
			Error  string          `json:"error"`
		}
		if err := doJSON(ctx, client, http.MethodGet, statusURL, h, nil, &st); err != nil {
			return nil, fmt.Errorf("firecrawl: %w", err)
		}
		switch st.Status {
		case "completed":
			pages = append(pages, st.Data...)
			// Large results are paginated through "next" (same API host only).
			next := st.Next
			for next != "" {
				nu, err := url.Parse(next)
				if err != nil || !strings.EqualFold(nu.Host, baseURL.Host) {
					break
				}
				var page struct {
					Data []firecrawlPage `json:"data"`
					Next string          `json:"next"`
				}
				if err := doJSON(ctx, client, http.MethodGet, next, h, nil, &page); err != nil {
					return nil, fmt.Errorf("firecrawl: %w", err)
				}
				pages = append(pages, page.Data...)
				next = page.Next
			}
			var docs []*document.Document
			for _, p := range pages {
				if p.Markdown == "" {
					continue
				}
				id := source
				if s, ok := p.Metadata["sourceURL"].(string); ok && s != "" {
					id = s
				}
				docs = append(docs, r.newDoc(source, id, p.Markdown, p.Metadata))
			}
			return docs, nil
		case "failed", "cancelled":
			return nil, fmt.Errorf("firecrawl: crawl %s %s: %s", start.ID, st.Status, st.Error)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("firecrawl: waiting for crawl %s: %w", start.ID, ctx.Err())
		case <-time.After(interval):
		}
	}
}

var _ Reader = (*FirecrawlReader)(nil)

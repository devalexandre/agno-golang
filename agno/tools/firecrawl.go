package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const firecrawlBaseURL = "https://api.firecrawl.dev/v1"

// FirecrawlTool provides web scraping with JS rendering and structured extraction via Firecrawl API.
type FirecrawlTool struct {
	toolkit.Toolkit
	apiKey     string
	httpClient *http.Client
}

// FirecrawlScrapeParams defines parameters for scraping a single page.
type FirecrawlScrapeParams struct {
	URL     string   `json:"url" description:"The URL to scrape." required:"true"`
	Formats []string `json:"formats,omitempty" description:"Output formats: markdown, html, rawHtml, links, screenshot. Default: [markdown]."`
}

// FirecrawlCrawlParams defines parameters for crawling multiple pages.
type FirecrawlCrawlParams struct {
	URL      string `json:"url" description:"The starting URL to crawl." required:"true"`
	MaxPages int    `json:"max_pages,omitempty" description:"Maximum number of pages to crawl. Default: 10."`
}

// FirecrawlMapParams defines parameters for generating a site map.
type FirecrawlMapParams struct {
	URL string `json:"url" description:"The URL to generate a sitemap for." required:"true"`
}

// NewFirecrawlTool creates a new Firecrawl tool.
// If apiKey is empty, it reads from the FIRECRAWL_API_KEY environment variable.
func NewFirecrawlTool(apiKey string) *FirecrawlTool {
	if apiKey == "" {
		apiKey = os.Getenv("FIRECRAWL_API_KEY")
	}

	t := &FirecrawlTool{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "FirecrawlTool"
	tk.Description = "Web scraping with JS rendering and structured extraction via Firecrawl."

	t.Toolkit = tk
	t.Toolkit.Register("Scrape", "Scrape a single web page with JS rendering.", t, t.Scrape, FirecrawlScrapeParams{})
	t.Toolkit.Register("Crawl", "Crawl multiple pages starting from a URL.", t, t.Crawl, FirecrawlCrawlParams{})
	t.Toolkit.Register("Map", "Generate a site map from a URL.", t, t.Map, FirecrawlMapParams{})

	return t
}

func (t *FirecrawlTool) doRequest(method, endpoint string, reqBody map[string]interface{}) (interface{}, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("FIRECRAWL_API_KEY not set")
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(method, firecrawlBaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("firecrawl request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firecrawl API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

// Scrape scrapes a single web page.
func (t *FirecrawlTool) Scrape(params FirecrawlScrapeParams) (interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	formats := params.Formats
	if len(formats) == 0 {
		formats = []string{"markdown"}
	}

	reqBody := map[string]interface{}{
		"url":     params.URL,
		"formats": formats,
	}

	return t.doRequest("POST", "/scrape", reqBody)
}

// Crawl crawls multiple pages starting from a URL.
func (t *FirecrawlTool) Crawl(params FirecrawlCrawlParams) (interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	maxPages := params.MaxPages
	if maxPages <= 0 {
		maxPages = 10
	}

	reqBody := map[string]interface{}{
		"url":   params.URL,
		"limit": maxPages,
	}

	return t.doRequest("POST", "/crawl", reqBody)
}

// Map generates a site map for a URL.
func (t *FirecrawlTool) Map(params FirecrawlMapParams) (interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	reqBody := map[string]interface{}{
		"url": params.URL,
	}

	return t.doRequest("POST", "/map", reqBody)
}

// Execute implements the toolkit.Tool interface.
func (t *FirecrawlTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

// Package scrapegraph ports Agno's ScrapeGraphTools: AI extraction, markdown
// conversion, raw HTML scraping, search and multi-page crawling through the
// ScrapeGraphAI v2 API (https://v2-api.scrapegraphai.com/api).
package scrapegraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the ScrapeGraphAI v2 API base URL.
const DefaultBaseURL = "https://v2-api.scrapegraphai.com/api"

// ScrapeGraphTools scrapes and extracts web content with ScrapeGraphAI.
type ScrapeGraphTools struct {
	toolkit.Toolkit
	apiKey              string
	baseURL             string
	httpClient          *http.Client
	client              *restkit.Client
	enableSmartscraper  bool
	enableMarkdownify   bool
	enableSearchscraper bool
	enableCrawl         bool
	enableScrape        bool
	all                 bool
	renderHeavyJS       bool
	headers             map[string]string
	crawlPollInterval   time.Duration
	crawlMaxWait        time.Duration
}

// Option configures ScrapeGraphTools.
type Option func(*ScrapeGraphTools)

// WithAPIKey sets the API key (default: SGAI_API_KEY).
func WithAPIKey(key string) Option { return func(s *ScrapeGraphTools) { s.apiKey = key } }

// WithBaseURL overrides the API base URL (default: SGAI_API_URL or DefaultBaseURL).
func WithBaseURL(u string) Option { return func(s *ScrapeGraphTools) { s.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(s *ScrapeGraphTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *ScrapeGraphTools) { s.httpClient = c } }

// WithEnableSmartscraper toggles Smartscraper (default true).
func WithEnableSmartscraper(v bool) Option {
	return func(s *ScrapeGraphTools) { s.enableSmartscraper = v }
}

// WithEnableMarkdownify toggles Markdownify (default false).
func WithEnableMarkdownify(v bool) Option {
	return func(s *ScrapeGraphTools) { s.enableMarkdownify = v }
}

// WithEnableSearchscraper toggles Searchscraper (default false).
func WithEnableSearchscraper(v bool) Option {
	return func(s *ScrapeGraphTools) { s.enableSearchscraper = v }
}

// WithEnableCrawl toggles Crawl (default false).
func WithEnableCrawl(v bool) Option { return func(s *ScrapeGraphTools) { s.enableCrawl = v } }

// WithEnableScrape toggles Scrape (default false).
func WithEnableScrape(v bool) Option { return func(s *ScrapeGraphTools) { s.enableScrape = v } }

// WithAll enables every tool.
func WithAll() Option { return func(s *ScrapeGraphTools) { s.all = true } }

// WithRenderHeavyJS requests JavaScript rendering on every call.
func WithRenderHeavyJS(v bool) Option { return func(s *ScrapeGraphTools) { s.renderHeavyJS = v } }

// WithHeaders sets custom HTTP headers sent with every outbound fetch.
func WithHeaders(h map[string]string) Option { return func(s *ScrapeGraphTools) { s.headers = h } }

// WithCrawlPollInterval sets the delay between crawl status polls (default 3s).
func WithCrawlPollInterval(d time.Duration) Option {
	return func(s *ScrapeGraphTools) { s.crawlPollInterval = d }
}

// WithCrawlMaxWait sets the maximum time to wait for a crawl (default 180s).
func WithCrawlMaxWait(d time.Duration) Option { return func(s *ScrapeGraphTools) { s.crawlMaxWait = d } }

// SmartscraperParams are the parameters of Smartscraper.
type SmartscraperParams struct {
	URL    string `json:"url" description:"The URL to scrape." required:"true"`
	Prompt string `json:"prompt" description:"Natural language prompt describing what to extract." required:"true"`
}

// URLParams carry a single URL.
type URLParams struct {
	URL string `json:"url" description:"The URL to process." required:"true"`
}

// SearchscraperParams are the parameters of Searchscraper.
type SearchscraperParams struct {
	Query string `json:"query" description:"The search query." required:"true"`
}

// CrawlParams are the parameters of Crawl.
type CrawlParams struct {
	URL      string                 `json:"url" description:"The URL to crawl." required:"true"`
	Prompt   string                 `json:"prompt" description:"Natural language prompt describing what to extract." required:"true"`
	Schema   map[string]interface{} `json:"schema" description:"JSON schema for extraction." required:"true"`
	MaxDepth int                    `json:"max_depth,omitempty" description:"Max crawl depth. Defaults to 2."`
	MaxPages int                    `json:"max_pages,omitempty" description:"Max number of pages to crawl. Defaults to 2."`
}

// New creates the ScrapeGraph toolkit.
func New(opts ...Option) *ScrapeGraphTools {
	s := &ScrapeGraphTools{
		enableSmartscraper: true,
		crawlPollInterval:  3 * time.Second,
		crawlMaxWait:       180 * time.Second,
	}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "scrapegraph_tools"
	s.Toolkit.Description = "Scrape, extract, search and crawl web content with ScrapeGraphAI."
	for _, o := range opts {
		o(s)
	}
	s.apiKey = restkit.Env(s.apiKey, "SGAI_API_KEY")
	s.baseURL = restkit.Env(s.baseURL, "SGAI_API_URL")
	if s.baseURL == "" {
		s.baseURL = DefaultBaseURL
	}
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: s.baseURL,
		Name:    "ScrapeGraphAI API",
		Headers: map[string]string{"SGAI-APIKEY": s.apiKey},
	}
	tk := &s.Toolkit
	if s.all || s.enableSmartscraper {
		tk.Register("Smartscraper", "Extract structured data from a webpage using an AI prompt.", s, s.Smartscraper, SmartscraperParams{})
	}
	if s.all || s.enableMarkdownify {
		tk.Register("Markdownify", "Convert a webpage to markdown.", s, s.Markdownify, URLParams{})
	}
	if s.all || s.enableSearchscraper {
		tk.Register("Searchscraper", "Search the web and extract information from the top results.", s, s.Searchscraper, SearchscraperParams{})
	}
	if s.all || s.enableCrawl {
		tk.Register("Crawl", "Crawl a website and extract structured data across multiple pages.", s, s.Crawl, CrawlParams{})
	}
	if s.all || s.enableScrape {
		tk.Register("Scrape", "Get raw HTML content from a webpage.", s, s.Scrape, URLParams{})
	}
	return s
}

type obj = map[string]interface{}

func (s *ScrapeGraphTools) fetchConfig() obj {
	fc := obj{}
	if s.renderHeavyJS {
		fc["mode"] = "js"
	}
	if len(s.headers) > 0 {
		fc["headers"] = s.headers
	}
	if len(fc) == 0 {
		return nil
	}
	return fc
}

func (s *ScrapeGraphTools) withFetch(body obj) obj {
	if fc := s.fetchConfig(); fc != nil {
		body["fetchConfig"] = fc
	}
	return body
}

// request performs a call and mirrors the SDK's ApiResult error messages.
func (s *ScrapeGraphTools) request(method, path string, body interface{}) (obj, string) {
	if s.apiKey == "" {
		return nil, "SGAI_API_KEY not set"
	}
	_, data, err := s.client.Raw(method, path, nil, body)
	if err != nil {
		var apiErr *restkit.APIError
		if errors.As(err, &apiErr) {
			msg := httpErrorMessage(apiErr.Status)
			var eb obj
			if json.Unmarshal(data, &eb) == nil {
				if d, ok := eb["detail"]; ok {
					if ds, ok := d.(string); ok {
						msg += ": " + ds
					} else {
						msg += ": " + restkit.CompactJSON(d)
					}
				}
			}
			return nil, msg
		}
		return nil, err.Error()
	}
	var out obj
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Sprintf("failed to decode response: %v", err)
	}
	return out, ""
}

func httpErrorMessage(status int) string {
	switch status {
	case 401:
		return "Invalid or missing API key"
	case 402:
		return "Insufficient credits - purchase more at https://dashboard.scrapegraphai.com"
	case 422:
		return "Invalid parameters - check your request"
	case 429:
		return "Rate limited - slow down and retry"
	case 500:
		return "Server error - try again later"
	default:
		return fmt.Sprintf("HTTP %d", status)
	}
}

// Smartscraper extracts structured data from a page with an AI prompt.
func (s *ScrapeGraphTools) Smartscraper(params SmartscraperParams) (interface{}, error) {
	res, errMsg := s.request(http.MethodPost, "/extract", s.withFetch(obj{"url": params.URL, "prompt": params.Prompt}))
	if errMsg != "" {
		return fmt.Sprintf("Error extracting from %s: %s", params.URL, errMsg), nil
	}
	if v, ok := res["json"]; ok && v != nil {
		return restkit.CompactJSON(v), nil
	}
	return restkit.CompactJSON(res["raw"]), nil
}

// Markdownify converts a page to markdown.
func (s *ScrapeGraphTools) Markdownify(params URLParams) (interface{}, error) {
	body := s.withFetch(obj{"url": params.URL, "formats": []obj{{"type": "markdown", "mode": "normal"}}})
	res, errMsg := s.request(http.MethodPost, "/scrape", body)
	if errMsg != "" {
		return fmt.Sprintf("Error converting %s to markdown: %s", params.URL, errMsg), nil
	}
	md := restkit.Map(res, "results")["markdown"]
	if m, ok := md.(map[string]interface{}); ok {
		if d, ok := m["data"]; ok && d != nil {
			if str, ok := d.(string); ok {
				return str, nil
			}
			return restkit.CompactJSON(d), nil
		}
		return "", nil
	}
	if md == nil {
		return "", nil
	}
	if str, ok := md.(string); ok {
		return str, nil
	}
	return restkit.CompactJSON(md), nil
}

// Searchscraper searches the web and extracts content from the top results.
func (s *ScrapeGraphTools) Searchscraper(params SearchscraperParams) (interface{}, error) {
	res, errMsg := s.request(http.MethodPost, "/search", s.withFetch(obj{"query": params.Query}))
	if errMsg != "" {
		return "Error searching: " + errMsg, nil
	}
	return restkit.CompactJSON(res), nil
}

// Crawl starts a crawl job and polls until it completes.
func (s *ScrapeGraphTools) Crawl(params CrawlParams) (interface{}, error) {
	maxDepth, maxPages := params.MaxDepth, params.MaxPages
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if maxPages <= 0 {
		maxPages = 2
	}
	body := s.withFetch(obj{
		"url":      params.URL,
		"formats":  []obj{{"type": "json", "prompt": params.Prompt, "schema": params.Schema, "mode": "normal"}},
		"maxDepth": maxDepth,
		"maxPages": maxPages,
	})
	data, errMsg := s.request(http.MethodPost, "/crawl", body)
	if errMsg != "" {
		return fmt.Sprintf("Error starting crawl of %s: %s", params.URL, errMsg), nil
	}
	crawlID := restkit.Str(data, "id")
	deadline := time.Now().Add(s.crawlMaxWait)
	for restkit.Str(data, "status") == "running" {
		if time.Now().After(deadline) {
			return fmt.Sprintf("Error: crawl timed out after %ds (id=%s)", int(s.crawlMaxWait.Seconds()), crawlID), nil
		}
		time.Sleep(s.crawlPollInterval)
		data, errMsg = s.request(http.MethodGet, "/crawl/"+url.PathEscape(crawlID), nil)
		if errMsg != "" {
			return fmt.Sprintf("Error polling crawl %s: %s", crawlID, errMsg), nil
		}
	}
	return restkit.CompactJSON(data), nil
}

// Scrape returns the raw HTML of a page.
func (s *ScrapeGraphTools) Scrape(params URLParams) (interface{}, error) {
	body := s.withFetch(obj{"url": params.URL, "formats": []obj{{"type": "html", "mode": "normal"}}})
	res, errMsg := s.request(http.MethodPost, "/scrape", body)
	if errMsg != "" {
		return fmt.Sprintf("Error scraping %s: %s", params.URL, errMsg), nil
	}
	return restkit.CompactJSON(res), nil
}

// Execute implements toolkit.Tool.
func (s *ScrapeGraphTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}

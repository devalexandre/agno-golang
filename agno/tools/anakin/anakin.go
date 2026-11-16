// Package anakin ports Agno's AnakinTools: single-URL scraping, multi-page
// crawling, site mapping and web search through the Anakin API
// (https://api.anakin.io/v1).
package anakin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Anakin API base URL.
const DefaultBaseURL = "https://api.anakin.io/v1"

// AnakinTools scrapes, crawls, maps and searches the web with Anakin.
type AnakinTools struct {
	toolkit.Toolkit
	apiKey       string
	baseURL      string
	country      string
	pollInterval time.Duration
	maxWaitTime  time.Duration
	enableScrape bool
	enableCrawl  bool
	enableMap    bool
	enableSearch bool
	all          bool
	httpClient   *http.Client
	client       *restkit.Client
}

// Option configures AnakinTools.
type Option func(*AnakinTools)

// WithAPIKey sets the API key (default: ANAKIN_API_KEY).
func WithAPIKey(key string) Option { return func(a *AnakinTools) { a.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(a *AnakinTools) { a.baseURL = u } }

// WithCountry sets the default two-letter proxy egress country (default "us").
func WithCountry(c string) Option { return func(a *AnakinTools) { a.country = c } }

// WithPollInterval sets the delay between job status checks (default 2s).
func WithPollInterval(d time.Duration) Option { return func(a *AnakinTools) { a.pollInterval = d } }

// WithMaxWaitTime sets the maximum time to wait for a job (default 120s).
func WithMaxWaitTime(d time.Duration) Option { return func(a *AnakinTools) { a.maxWaitTime = d } }

// WithTimeout sets the HTTP timeout per request (default 60s).
func WithTimeout(d time.Duration) Option { return func(a *AnakinTools) { a.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(a *AnakinTools) { a.httpClient = c } }

// WithEnableScrape toggles ScrapeUrl (default true).
func WithEnableScrape(v bool) Option { return func(a *AnakinTools) { a.enableScrape = v } }

// WithEnableCrawl toggles CrawlWebsite (default false).
func WithEnableCrawl(v bool) Option { return func(a *AnakinTools) { a.enableCrawl = v } }

// WithEnableMap toggles MapWebsite (default false).
func WithEnableMap(v bool) Option { return func(a *AnakinTools) { a.enableMap = v } }

// WithEnableSearch toggles SearchWeb (default false).
func WithEnableSearch(v bool) Option { return func(a *AnakinTools) { a.enableSearch = v } }

// WithAll enables every tool.
func WithAll() Option { return func(a *AnakinTools) { a.all = true } }

// ScrapeURLParams are the parameters of ScrapeUrl.
type ScrapeURLParams struct {
	URL          string `json:"url" description:"The URL to scrape." required:"true"`
	GenerateJSON bool   `json:"generate_json,omitempty" description:"Also extract structured JSON from the page with AI. Defaults to false."`
	UseBrowser   bool   `json:"use_browser,omitempty" description:"Render with a stealth headless browser, for SPAs and JS-heavy pages. Defaults to false."`
	Country      string `json:"country,omitempty" description:"Two-letter proxy egress country code. Defaults to the toolkit's country."`
}

// CrawlWebsiteParams are the parameters of CrawlWebsite.
type CrawlWebsiteParams struct {
	URL             string   `json:"url" description:"The starting URL to crawl from." required:"true"`
	MaxPages        int      `json:"max_pages,omitempty" description:"Maximum number of pages to fetch. Defaults to 10."`
	Depth           int      `json:"depth,omitempty" description:"Link-hops from the starting URL to follow. Defaults to 1."`
	IncludePatterns []string `json:"include_patterns,omitempty" description:"Only fetch URLs matching one of these glob patterns."`
	ExcludePatterns []string `json:"exclude_patterns,omitempty" description:"Skip URLs matching any of these glob patterns."`
	UseBrowser      bool     `json:"use_browser,omitempty" description:"Render each page with a headless browser, for SPAs. Defaults to false."`
	Country         string   `json:"country,omitempty" description:"Two-letter proxy egress country code. Defaults to the toolkit's country."`
}

// MapWebsiteParams are the parameters of MapWebsite.
type MapWebsiteParams struct {
	URL                  string `json:"url" description:"The starting URL for discovery, typically a homepage or section root." required:"true"`
	Limit                int    `json:"limit,omitempty" description:"Maximum number of URLs to return overall. Defaults to 100."`
	Depth                int    `json:"depth,omitempty" description:"Link-hops from the starting URL to follow. Defaults to 2."`
	LimitPerLevel        int    `json:"limit_per_level,omitempty" description:"Maximum URLs collected per depth level. Defaults to 100."`
	IncludeExternalLinks bool   `json:"include_external_links,omitempty" description:"Also collect (but do not follow) external links. Defaults to false."`
	IncludeSubdomains    bool   `json:"include_subdomains,omitempty" description:"Include URLs on subdomains of the starting host. Defaults to false."`
	Search               string `json:"search,omitempty" description:"Only return URLs whose path/title matches this keyword."`
	UseBrowser           bool   `json:"use_browser,omitempty" description:"Render with a headless browser, for SPAs. Defaults to false."`
}

// SearchWebParams are the parameters of SearchWeb.
type SearchWebParams struct {
	Query string `json:"query" description:"The search query, in natural language." required:"true"`
	Limit int    `json:"limit,omitempty" description:"Maximum number of results to return. Defaults to 5."`
}

// New creates the Anakin toolkit.
func New(opts ...Option) *AnakinTools {
	a := &AnakinTools{
		baseURL:      DefaultBaseURL,
		country:      "us",
		pollInterval: 2 * time.Second,
		maxWaitTime:  120 * time.Second,
		enableScrape: true,
	}
	a.Toolkit = toolkit.NewToolkit()
	a.Toolkit.Name = "anakin_tools"
	a.Toolkit.Description = "Scrape, crawl, map and search the web using the Anakin API."
	a.Toolkit.Timeout = 60 * time.Second
	for _, o := range opts {
		o(a)
	}
	a.apiKey = restkit.Env(a.apiKey, "ANAKIN_API_KEY")
	if a.httpClient == nil {
		a.httpClient = a.Toolkit.HTTPClient()
	}
	a.client = &restkit.Client{
		HTTP:    a.httpClient,
		BaseURL: a.baseURL,
		Name:    "Anakin API",
		Headers: map[string]string{"Authorization": "Bearer " + a.apiKey},
	}
	if a.all || a.enableScrape {
		a.Toolkit.Register("ScrapeUrl", "Scrape a single URL using Anakin and return the markdown (and structured data if requested) as JSON.", a, a.ScrapeUrl, ScrapeURLParams{})
	}
	if a.all || a.enableCrawl {
		a.Toolkit.Register("CrawlWebsite", "Crawl a website using Anakin, fetching markdown for every page reached.", a, a.CrawlWebsite, CrawlWebsiteParams{})
	}
	if a.all || a.enableMap {
		a.Toolkit.Register("MapWebsite", "Discover URLs reachable from a starting URL using Anakin.", a, a.MapWebsite, MapWebsiteParams{})
	}
	if a.all || a.enableSearch {
		a.Toolkit.Register("SearchWeb", "Search the web using Anakin, returning result URLs, titles and snippets.", a, a.SearchWeb, SearchWebParams{})
	}
	return a
}

// withoutNil drops nil values, mirroring the Python payload filter.
func withoutNil(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		switch x := v.(type) {
		case nil:
			delete(m, k)
		case []string:
			if x == nil {
				delete(m, k)
			}
		case string:
			if x == "" {
				delete(m, k)
			}
		}
	}
	return m
}

// runJob submits an async job and polls it until completion.
func (a *AnakinTools) runJob(path string, payload map[string]interface{}) (map[string]interface{}, error) {
	var submitted map[string]interface{}
	if err := a.client.Into(http.MethodPost, path, nil, withoutNil(payload), &submitted); err != nil {
		return nil, err
	}
	jobID := restkit.Str(submitted, "jobId")
	if jobID == "" {
		jobID = restkit.Str(submitted, "id")
	}
	if jobID == "" {
		return nil, fmt.Errorf("Anakin did not return a job id: %s", restkit.CompactJSON(submitted))
	}
	deadline := time.Now().Add(a.maxWaitTime)
	for {
		var result map[string]interface{}
		if err := a.client.Into(http.MethodGet, path+"/"+jobID, nil, nil, &result); err != nil {
			return nil, err
		}
		status := restkit.Str(result, "status")
		switch status {
		case "completed":
			return result, nil
		case "failed":
			msg := restkit.Str(result, "error")
			if msg == "" {
				msg = "unknown error"
			}
			return nil, fmt.Errorf("Job %s failed: %s", jobID, msg)
		}
		if time.Now().Add(a.pollInterval).After(deadline) {
			return nil, fmt.Errorf("Job %s was still '%s' after %gs. Retry later or raise max_wait_time.", jobID, status, a.maxWaitTime.Seconds())
		}
		time.Sleep(a.pollInterval)
	}
}

func errText(prefix string, err error) string {
	var apiErr *restkit.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("%s: %d %s", prefix, apiErr.Status, apiErr.Body)
	}
	return fmt.Sprintf("%s: %v", prefix, err)
}

func (a *AnakinTools) countryOr(c string) string {
	if c != "" {
		return c
	}
	return a.country
}

// ScrapeUrl scrapes a single URL.
func (a *AnakinTools) ScrapeUrl(params ScrapeURLParams) (interface{}, error) {
	if a.apiKey == "" {
		return "Error: ANAKIN_API_KEY not set", nil
	}
	result, err := a.runJob("/url-scraper", map[string]interface{}{
		"url":          params.URL,
		"generateJson": params.GenerateJSON,
		"useBrowser":   params.UseBrowser,
		"country":      a.countryOr(params.Country),
	})
	if err != nil {
		return errText("Error scraping "+params.URL, err), nil
	}
	return restkit.CompactJSON(result), nil
}

// CrawlWebsite crawls a website starting from a URL.
func (a *AnakinTools) CrawlWebsite(params CrawlWebsiteParams) (interface{}, error) {
	if a.apiKey == "" {
		return "Error: ANAKIN_API_KEY not set", nil
	}
	maxPages := params.MaxPages
	if maxPages <= 0 {
		maxPages = 10
	}
	depth := params.Depth
	if depth <= 0 {
		depth = 1
	}
	result, err := a.runJob("/crawl", map[string]interface{}{
		"url":             params.URL,
		"maxPages":        maxPages,
		"depth":           depth,
		"includePatterns": params.IncludePatterns,
		"excludePatterns": params.ExcludePatterns,
		"useBrowser":      params.UseBrowser,
		"country":         a.countryOr(params.Country),
	})
	if err != nil {
		return errText("Error crawling "+params.URL, err), nil
	}
	return restkit.CompactJSON(result), nil
}

// MapWebsite discovers URLs reachable from a starting URL.
func (a *AnakinTools) MapWebsite(params MapWebsiteParams) (interface{}, error) {
	if a.apiKey == "" {
		return "Error: ANAKIN_API_KEY not set", nil
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	depth := params.Depth
	if depth <= 0 {
		depth = 2
	}
	perLevel := params.LimitPerLevel
	if perLevel <= 0 {
		perLevel = 100
	}
	result, err := a.runJob("/map", map[string]interface{}{
		"url":                  params.URL,
		"limit":                limit,
		"depth":                depth,
		"limitPerLevel":        perLevel,
		"includeExternalLinks": params.IncludeExternalLinks,
		"includeSubdomains":    params.IncludeSubdomains,
		"search":               params.Search,
		"useBrowser":           params.UseBrowser,
	})
	if err != nil {
		return errText("Error mapping "+params.URL, err), nil
	}
	return restkit.CompactJSON(result), nil
}

// SearchWeb searches the web.
func (a *AnakinTools) SearchWeb(params SearchWebParams) (interface{}, error) {
	if a.apiKey == "" {
		return "Error: ANAKIN_API_KEY not set", nil
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 5
	}
	result, err := a.client.JSON(http.MethodPost, "/search", nil, map[string]interface{}{"prompt": params.Query, "limit": limit})
	if err != nil {
		return errText("Error searching for "+params.Query, err), nil
	}
	return restkit.CompactJSON(result), nil
}

// Execute implements toolkit.Tool.
func (a *AnakinTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return a.Toolkit.Execute(methodName, input)
}

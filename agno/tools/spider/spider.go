// Package spider ports Agno's SpiderTools: web search, scraping and crawling
// through the Spider API (https://api.spider.cloud).
package spider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Spider API base URL.
const DefaultBaseURL = "https://api.spider.cloud"

// SpiderTools searches, scrapes and crawls the web with Spider.
type SpiderTools struct {
	toolkit.Toolkit
	apiKey         string
	baseURL        string
	maxResults     int
	optionalParams map[string]interface{}
	enableSearch   bool
	enableScrape   bool
	enableCrawl    bool
	all            bool
	httpClient     *http.Client
	client         *restkit.Client
}

// Option configures SpiderTools.
type Option func(*SpiderTools)

// WithAPIKey sets the API key (default: SPIDER_API_KEY).
func WithAPIKey(key string) Option { return func(s *SpiderTools) { s.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(s *SpiderTools) { s.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(s *SpiderTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SpiderTools) { s.httpClient = c } }

// WithMaxResults sets the default number of search results (Python max_results; default 5).
func WithMaxResults(n int) Option { return func(s *SpiderTools) { s.maxResults = n } }

// WithOptionalParams sets extra request parameters merged into every call.
func WithOptionalParams(p map[string]interface{}) Option {
	return func(s *SpiderTools) { s.optionalParams = p }
}

// WithEnableSearch toggles SearchWeb (default true).
func WithEnableSearch(v bool) Option { return func(s *SpiderTools) { s.enableSearch = v } }

// WithEnableScrape toggles Scrape (default true).
func WithEnableScrape(v bool) Option { return func(s *SpiderTools) { s.enableScrape = v } }

// WithEnableCrawl toggles Crawl (default true).
func WithEnableCrawl(v bool) Option { return func(s *SpiderTools) { s.enableCrawl = v } }

// WithAll enables every tool.
func WithAll() Option { return func(s *SpiderTools) { s.all = true } }

// SearchWebParams are the parameters of SearchWeb.
type SearchWebParams struct {
	Query      string `json:"query" description:"The query to search the web with." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"The maximum number of results to return. Defaults to the value set on the toolkit, or 5."`
}

// ScrapeParams are the parameters of Scrape.
type ScrapeParams struct {
	URL string `json:"url" description:"The URL of the webpage to scrape." required:"true"`
}

// CrawlParams are the parameters of Crawl.
type CrawlParams struct {
	URL   string `json:"url" description:"The URL of the webpage to crawl." required:"true"`
	Limit int    `json:"limit,omitempty" description:"The maximum number of pages to crawl. Defaults to 10."`
}

// New creates the Spider toolkit.
func New(opts ...Option) *SpiderTools {
	s := &SpiderTools{baseURL: DefaultBaseURL, enableSearch: true, enableScrape: true, enableCrawl: true}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "spider"
	s.Toolkit.Description = "Search, scrape and crawl the web using Spider."
	for _, o := range opts {
		o(s)
	}
	s.apiKey = restkit.Env(s.apiKey, "SPIDER_API_KEY")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: s.baseURL,
		Name:    "Spider API",
		Headers: map[string]string{"Authorization": "Bearer " + s.apiKey},
	}
	if s.all || s.enableSearch {
		s.Toolkit.Register("SearchWeb", "Use this function to search the web.", s, s.SearchWeb, SearchWebParams{})
	}
	if s.all || s.enableScrape {
		s.Toolkit.Register("Scrape", "Use this function to scrape the content of a webpage. Returns markdown of the webpage.", s, s.Scrape, ScrapeParams{})
	}
	if s.all || s.enableCrawl {
		s.Toolkit.Register("Crawl", "Use this function to crawl the web.", s, s.Crawl, CrawlParams{})
	}
	return s
}

func (s *SpiderTools) post(endpoint string, body map[string]interface{}) (string, error) {
	if s.apiKey == "" {
		return "", fmt.Errorf("SPIDER_API_KEY is required")
	}
	res, err := s.client.JSON(http.MethodPost, endpoint, nil, body)
	if err != nil {
		return "", err
	}
	return restkit.CompactJSON(res), nil
}

func (s *SpiderTools) merge(base map[string]interface{}) map[string]interface{} {
	for k, v := range s.optionalParams {
		base[k] = v
	}
	return base
}

// SearchWeb searches the web with Spider.
func (s *SpiderTools) SearchWeb(params SearchWebParams) (interface{}, error) {
	num := 5
	if s.maxResults > 0 {
		num = s.maxResults
	}
	body := s.merge(map[string]interface{}{"fetch_page_content": false, "num": num})
	if params.MaxResults > 0 {
		body["num"] = params.MaxResults
	}
	body["search"] = params.Query
	out, err := s.post("search", body)
	if err != nil {
		return fmt.Sprintf("Error fetching results from spider: %v", err), nil
	}
	return out, nil
}

// Scrape returns the markdown of a single page.
func (s *SpiderTools) Scrape(params ScrapeParams) (interface{}, error) {
	body := s.merge(map[string]interface{}{"return_format": "markdown"})
	body["url"] = params.URL
	body["limit"] = 1
	out, err := s.post("crawl", body)
	if err != nil {
		return fmt.Sprintf("Error fetching content from spider: %v", err), nil
	}
	return out, nil
}

// Crawl crawls a website starting at url.
func (s *SpiderTools) Crawl(params CrawlParams) (interface{}, error) {
	body := s.merge(map[string]interface{}{"return_format": "markdown", "limit": 10})
	if params.Limit > 0 {
		body["limit"] = params.Limit
	}
	body["url"] = params.URL
	out, err := s.post("crawl", body)
	if err != nil {
		return fmt.Sprintf("Error fetching content from spider: %v", err), nil
	}
	return out, nil
}

// Execute implements toolkit.Tool.
func (s *SpiderTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}

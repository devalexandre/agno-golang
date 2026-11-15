// Package bravesearch ports Agno's BraveSearchTools: web search through the
// Brave Search API (https://api.search.brave.com).
package bravesearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Brave Search API base URL.
const DefaultBaseURL = "https://api.search.brave.com/res/v1"

// BraveSearchTools searches the web with Brave.
type BraveSearchTools struct {
	toolkit.Toolkit
	apiKey          string
	baseURL         string
	fixedMaxResults int
	fixedLanguage   string
	httpClient      *http.Client
	client          *restkit.Client
}

// Option configures BraveSearchTools.
type Option func(*BraveSearchTools)

// WithAPIKey sets the API key (default: BRAVE_API_KEY).
func WithAPIKey(key string) Option { return func(b *BraveSearchTools) { b.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(b *BraveSearchTools) { b.baseURL = u } }

// WithFixedMaxResults forces the number of results regardless of the call argument.
func WithFixedMaxResults(n int) Option { return func(b *BraveSearchTools) { b.fixedMaxResults = n } }

// WithFixedLanguage forces the search language regardless of the call argument.
func WithFixedLanguage(lang string) Option { return func(b *BraveSearchTools) { b.fixedLanguage = lang } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(b *BraveSearchTools) { b.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(b *BraveSearchTools) { b.httpClient = c } }

// SearchParams are the parameters of BraveSearch.
type SearchParams struct {
	Query      string `json:"query" description:"The query to search for." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"The maximum number of results to return. Default is 5."`
	Country    string `json:"country,omitempty" description:"The country code for search results. Default is US."`
	SearchLang string `json:"search_lang,omitempty" description:"The language of the search results. Default is en."`
}

// New creates the Brave Search toolkit.
func New(opts ...Option) *BraveSearchTools {
	b := &BraveSearchTools{baseURL: DefaultBaseURL}
	b.Toolkit = toolkit.NewToolkit()
	b.Toolkit.Name = "brave_search"
	b.Toolkit.Description = "Search the web using the Brave Search API."
	for _, o := range opts {
		o(b)
	}
	b.apiKey = restkit.Env(b.apiKey, "BRAVE_API_KEY")
	if b.httpClient == nil {
		b.httpClient = b.Toolkit.HTTPClient()
	}
	b.client = &restkit.Client{
		HTTP:    b.httpClient,
		BaseURL: b.baseURL,
		Name:    "Brave API",
		Headers: map[string]string{"X-Subscription-Token": b.apiKey},
	}
	b.Toolkit.Register("BraveSearch", "Search Brave for the specified query and return the results as JSON.", b, b.BraveSearch, SearchParams{})
	return b
}

// BraveSearch searches Brave and returns {web_results, query, total_results}.
func (b *BraveSearchTools) BraveSearch(params SearchParams) (interface{}, error) {
	if params.Query == "" {
		return restkit.ErrorJSON("Please provide a query to search for"), nil
	}
	if b.apiKey == "" {
		return nil, fmt.Errorf("BRAVE_API_KEY is required")
	}
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if b.fixedMaxResults > 0 {
		maxResults = b.fixedMaxResults
	}
	lang := params.SearchLang
	if lang == "" {
		lang = "en"
	}
	if b.fixedLanguage != "" {
		lang = b.fixedLanguage
	}
	country := params.Country
	if country == "" {
		country = "US"
	}
	q := url.Values{
		"q":             {params.Query},
		"count":         {strconv.Itoa(maxResults)},
		"country":       {country},
		"search_lang":   {lang},
		"result_filter": {"web"},
	}
	var resp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := b.client.Into(http.MethodGet, "/web/search", q, nil, &resp); err != nil {
		return nil, err
	}
	type webResult struct {
		Title       string `json:"title"`
		URL         string `json:"url"`
		Description string `json:"description"`
	}
	results := []webResult{}
	for _, r := range resp.Web.Results {
		results = append(results, webResult{Title: r.Title, URL: r.URL, Description: r.Description})
	}
	return restkit.JSONString(map[string]interface{}{
		"web_results":   results,
		"query":         params.Query,
		"total_results": len(results),
	}), nil
}

// Execute implements toolkit.Tool.
func (b *BraveSearchTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return b.Toolkit.Execute(methodName, input)
}

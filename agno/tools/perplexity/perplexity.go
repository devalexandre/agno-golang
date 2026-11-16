// Package perplexity ports Agno's PerplexitySearch: ranked raw web search
// results through the Perplexity Search API (POST https://api.perplexity.ai/search).
package perplexity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Perplexity API base URL.
const DefaultBaseURL = "https://api.perplexity.ai"

// PerplexitySearch searches the web with the Perplexity Search API.
type PerplexitySearch struct {
	toolkit.Toolkit
	apiKey               string
	baseURL              string
	maxResults           int
	maxTokensPerPage     int
	searchRecencyFilter  string
	searchDomainFilter   []string
	searchLanguageFilter []string
	httpClient           *http.Client
	client               *restkit.Client
}

// Option configures PerplexitySearch.
type Option func(*PerplexitySearch)

// WithAPIKey sets the API key (default: PERPLEXITY_API_KEY).
func WithAPIKey(key string) Option { return func(p *PerplexitySearch) { p.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(p *PerplexitySearch) { p.baseURL = u } }

// WithMaxResults sets the default number of results (default 5).
func WithMaxResults(n int) Option { return func(p *PerplexitySearch) { p.maxResults = n } }

// WithMaxTokensPerPage sets the max tokens of content per result page (default 2048).
func WithMaxTokensPerPage(n int) Option { return func(p *PerplexitySearch) { p.maxTokensPerPage = n } }

// WithSearchRecencyFilter filters by recency ("day", "week", "month", "year").
func WithSearchRecencyFilter(f string) Option {
	return func(p *PerplexitySearch) { p.searchRecencyFilter = f }
}

// WithSearchDomainFilter restricts/excludes domains.
func WithSearchDomainFilter(d ...string) Option {
	return func(p *PerplexitySearch) { p.searchDomainFilter = d }
}

// WithSearchLanguageFilter filters by ISO language codes.
func WithSearchLanguageFilter(l ...string) Option {
	return func(p *PerplexitySearch) { p.searchLanguageFilter = l }
}

// WithTimeout sets the HTTP timeout (Python uses 30s).
func WithTimeout(d time.Duration) Option { return func(p *PerplexitySearch) { p.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(p *PerplexitySearch) { p.httpClient = c } }

// SearchParams are the parameters of Search.
type SearchParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Number of results to return. Defaults to instance setting."`
}

// New creates the Perplexity Search toolkit.
func New(opts ...Option) *PerplexitySearch {
	p := &PerplexitySearch{baseURL: DefaultBaseURL, maxResults: 5, maxTokensPerPage: 2048}
	p.Toolkit = toolkit.NewToolkit()
	p.Toolkit.Name = "perplexity_search"
	p.Toolkit.Description = "Search the web using the Perplexity Search API."
	for _, o := range opts {
		o(p)
	}
	p.apiKey = restkit.Env(p.apiKey, "PERPLEXITY_API_KEY")
	if p.httpClient == nil {
		p.httpClient = p.Toolkit.HTTPClient()
	}
	p.client = &restkit.Client{
		HTTP:    p.httpClient,
		BaseURL: p.baseURL,
		Name:    "Perplexity API",
		Headers: map[string]string{
			"Authorization":      "Bearer " + p.apiKey,
			"X-Source":           "agno",
			"X-Pplx-Integration": "agno",
		},
	}
	p.Toolkit.Register("Search", "Use this function to search the web using the Perplexity Search API. Returns ranked web search results with titles, URLs, snippets, and dates.", p, p.Search, SearchParams{})
	return p
}

type result struct {
	URL     string `json:"url"`
	Title   string `json:"title,omitempty"`
	Snippet string `json:"snippet,omitempty"`
	Date    string `json:"date,omitempty"`
}

// Search returns a JSON array (indent 4) of {url, title, snippet, date}, or {"error": ...}.
func (p *PerplexitySearch) Search(params SearchParams) (interface{}, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("PERPLEXITY_API_KEY is required")
	}
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = p.maxResults
	}
	body := map[string]interface{}{
		"query":               params.Query,
		"max_results":         maxResults,
		"max_tokens_per_page": p.maxTokensPerPage,
	}
	if p.searchRecencyFilter != "" {
		body["search_recency_filter"] = p.searchRecencyFilter
	}
	if len(p.searchDomainFilter) > 0 {
		body["search_domain_filter"] = p.searchDomainFilter
	}
	if len(p.searchLanguageFilter) > 0 {
		body["search_language_filter"] = p.searchLanguageFilter
	}
	var resp struct {
		Results []struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
			Date    string `json:"date"`
		} `json:"results"`
	}
	if err := p.client.Into(http.MethodPost, "/search", nil, body, &resp); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	results := []result{}
	for _, r := range resp.Results {
		results = append(results, result{URL: r.URL, Title: r.Title, Snippet: r.Snippet, Date: r.Date})
	}
	return dumps(results), nil
}

// dumps mirrors json.dumps(v, indent=4, ensure_ascii=False).
func dumps(v interface{}) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimRight(buf.String(), "\n")
}

// Execute implements toolkit.Tool.
func (p *PerplexitySearch) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return p.Toolkit.Execute(methodName, input)
}

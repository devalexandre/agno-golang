// Package seltz ports Agno's SeltzTools: AI-powered web search through the
// Seltz API. The Python SDK talks gRPC; this port uses Seltz's REST endpoint
// (POST https://api.seltz.ai/v1/search) with the same request fields.
package seltz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Seltz REST API base URL.
const DefaultBaseURL = "https://api.seltz.ai"

// SeltzTools searches the web with Seltz.
type SeltzTools struct {
	toolkit.Toolkit
	apiKey       string
	baseURL      string
	maxResults   int
	enableSearch bool
	all          bool
	httpClient   *http.Client
	client       *restkit.Client
}

// Option configures SeltzTools.
type Option func(*SeltzTools)

// WithAPIKey sets the API key (default: SELTZ_API_KEY).
func WithAPIKey(key string) Option { return func(s *SeltzTools) { s.apiKey = key } }

// WithBaseURL overrides the API base URL (Python's endpoint option).
func WithBaseURL(u string) Option { return func(s *SeltzTools) { s.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(s *SeltzTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SeltzTools) { s.httpClient = c } }

// WithMaxResults sets the default maximum number of results (default 10).
func WithMaxResults(n int) Option { return func(s *SeltzTools) { s.maxResults = n } }

// WithEnableSearch toggles SearchSeltz (default true).
func WithEnableSearch(v bool) Option { return func(s *SeltzTools) { s.enableSearch = v } }

// WithAll enables every tool.
func WithAll() Option { return func(s *SeltzTools) { s.all = true } }

// SearchParams are the parameters of SearchSeltz.
type SearchParams struct {
	Query          string   `json:"query" description:"The query to search for." required:"true"`
	MaxResults     int      `json:"max_results,omitempty" description:"Maximum number of results to return. Defaults to toolkit max_results."`
	Scope          string   `json:"scope,omitempty" description:"Restrict the search to a supported scope, such as \"news\"."`
	IncludeDomains []string `json:"include_domains,omitempty" description:"Only include results from these domains."`
	ExcludeDomains []string `json:"exclude_domains,omitempty" description:"Exclude results from these domains."`
	FromDate       string   `json:"from_date,omitempty" description:"Only include results published on or after this ISO 8601 date."`
	ToDate         string   `json:"to_date,omitempty" description:"Only include results published on or before this ISO 8601 date."`
	Context        string   `json:"context,omitempty" description:"Legacy SDK context. Ignored by current Seltz versions."`
}

// New creates the Seltz toolkit.
func New(opts ...Option) *SeltzTools {
	s := &SeltzTools{baseURL: DefaultBaseURL, enableSearch: true}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "seltz"
	s.Toolkit.Description = "Search the web with the Seltz AI-powered search API."
	for _, o := range opts {
		o(s)
	}
	if s.maxResults <= 0 {
		s.maxResults = 10
	}
	s.apiKey = restkit.Env(s.apiKey, "SELTZ_API_KEY")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: s.baseURL,
		Name:    "Seltz API",
		Headers: map[string]string{"x-api-key": s.apiKey},
	}
	if s.all || s.enableSearch {
		s.Toolkit.Register("SearchSeltz", "Use this function to search Seltz for a query. Returns search results in JSON format.", s, s.SearchSeltz, SearchParams{})
	}
	return s
}

// SearchSeltz searches Seltz and returns [{url, content}] as JSON.
func (s *SeltzTools) SearchSeltz(params SearchParams) (interface{}, error) {
	if params.Query == "" {
		return "Error: Please provide a query to search for.", nil
	}
	if s.apiKey == "" {
		return "Error: SELTZ_API_KEY not set. Please set the SELTZ_API_KEY environment variable.", nil
	}
	limit := s.maxResults
	if params.MaxResults < 0 {
		return "Error: max_results must be greater than 0.", nil
	}
	if params.MaxResults > 0 {
		limit = params.MaxResults
	}
	body := map[string]interface{}{"query": params.Query, "max_results": limit}
	if params.Scope != "" {
		body["scope"] = params.Scope
	}
	if params.IncludeDomains != nil {
		body["include_domains"] = params.IncludeDomains
	}
	if params.ExcludeDomains != nil {
		body["exclude_domains"] = params.ExcludeDomains
	}
	if params.FromDate != "" {
		body["from_date"] = params.FromDate
	}
	if params.ToDate != "" {
		body["to_date"] = params.ToDate
	}
	var resp struct {
		Documents []struct {
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"documents"`
	}
	if err := s.client.Into(http.MethodPost, "/v1/search", nil, body, &resp); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	parsed := []map[string]string{}
	for _, d := range resp.Documents {
		doc := map[string]string{}
		if d.URL != "" {
			doc["url"] = d.URL
		}
		if d.Content != "" {
			doc["content"] = d.Content
		}
		if len(doc) > 0 {
			parsed = append(parsed, doc)
		}
	}
	return indent4(parsed), nil
}

// indent4 mirrors json.dumps(..., indent=4, ensure_ascii=False).
func indent4(v interface{}) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}

// Execute implements toolkit.Tool.
func (s *SeltzTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}

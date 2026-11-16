// Package serply ports Agno's SerplyTools: Google web, Google News and Google
// Scholar search through the Serply API (https://api.serply.io/v1).
package serply

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Serply API base URL.
const DefaultBaseURL = "https://api.serply.io/v1"

// SerplyTools searches Google, Google News and Google Scholar with Serply.
type SerplyTools struct {
	toolkit.Toolkit
	apiKey        string
	baseURL       string
	numResults    int
	enableWeb     bool
	enableNews    bool
	enableScholar bool
	all           bool
	httpClient    *http.Client
	client        *restkit.Client
}

// Option configures SerplyTools.
type Option func(*SerplyTools)

// WithAPIKey sets the API key (default: SERPLY_API_KEY).
func WithAPIKey(key string) Option { return func(s *SerplyTools) { s.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(s *SerplyTools) { s.baseURL = u } }

// WithNumResults sets the default number of results (default 10).
func WithNumResults(n int) Option { return func(s *SerplyTools) { s.numResults = n } }

// WithEnableSearchWeb toggles SearchWeb (default true).
func WithEnableSearchWeb(b bool) Option { return func(s *SerplyTools) { s.enableWeb = b } }

// WithEnableSearchNews toggles SearchNews (default false).
func WithEnableSearchNews(b bool) Option { return func(s *SerplyTools) { s.enableNews = b } }

// WithEnableSearchScholar toggles SearchScholar (default false).
func WithEnableSearchScholar(b bool) Option { return func(s *SerplyTools) { s.enableScholar = b } }

// WithAll enables every tool.
func WithAll() Option { return func(s *SerplyTools) { s.all = true } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(s *SerplyTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SerplyTools) { s.httpClient = c } }

// SearchParams are the parameters of every Serply search method.
type SearchParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of results to return. Defaults to instance setting."`
}

// New creates the Serply toolkit.
func New(opts ...Option) *SerplyTools {
	s := &SerplyTools{baseURL: DefaultBaseURL, numResults: 10, enableWeb: true}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "serply_tools"
	s.Toolkit.Description = "Search Google web, Google News and Google Scholar using the Serply API."
	for _, o := range opts {
		o(s)
	}
	s.apiKey = restkit.Env(s.apiKey, "SERPLY_API_KEY")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: s.baseURL,
		Name:    "Serply API",
		Headers: map[string]string{"X-Api-Key": s.apiKey, "User-Agent": "agno"},
	}
	if s.all || s.enableWeb {
		s.Toolkit.Register("SearchWeb", "Search Google for the given query using Serply. Returns organic results and related searches as JSON.", s, s.SearchWeb, SearchParams{})
	}
	if s.all || s.enableNews {
		s.Toolkit.Register("SearchNews", "Search Google News for the given query using Serply. Returns news articles with title, link, source and publish date as JSON.", s, s.SearchNews, SearchParams{})
	}
	if s.all || s.enableScholar {
		s.Toolkit.Register("SearchScholar", "Search Google Scholar for academic papers using Serply. Returns papers with title, link, authors, citation count and PDF link as JSON.", s, s.SearchScholar, SearchParams{})
	}
	return s
}

func (s *SerplyTools) num(n int) int {
	if n > 0 {
		return n
	}
	return s.numResults
}

// request performs GET /{endpoint}/ and returns the decoded body or an error message.
func (s *SerplyTools) request(endpoint, query string, num int) (map[string]interface{}, string) {
	if s.apiKey == "" {
		return nil, "No Serply API key provided. Set the SERPLY_API_KEY environment variable."
	}
	var data map[string]interface{}
	q := url.Values{"q": {query}, "num": {strconv.Itoa(num)}}
	if err := s.client.Into(http.MethodGet, "/"+endpoint+"/", q, nil, &data); err != nil {
		if _, ok := err.(*restkit.APIError); ok {
			return nil, "HTTP error: " + err.Error()
		}
		return nil, err.Error()
	}
	return data, ""
}

func obj(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func get(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}

func list(m map[string]interface{}, key string) []interface{} {
	arr, _ := get(m, key).([]interface{})
	return arr
}

// SearchWeb searches Google.
func (s *SerplyTools) SearchWeb(p SearchParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON("Please provide a query to search for"), nil
	}
	data, errMsg := s.request("search", p.Query, s.num(p.NumResults))
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type item struct {
		Position    interface{} `json:"position"`
		Title       interface{} `json:"title"`
		Link        interface{} `json:"link"`
		Description interface{} `json:"description"`
	}
	results := []item{}
	for _, it := range list(data, "results") {
		r := obj(it)
		results = append(results, item{get(r, "position"), get(r, "title"), get(r, "link"), get(r, "description")})
	}
	related := get(data, "related_searches")
	if related == nil {
		related = []interface{}{}
	}
	return restkit.JSONString(struct {
		Results         []item      `json:"results"`
		RelatedSearches interface{} `json:"related_searches"`
	}{results, related}), nil
}

// SearchNews searches Google News (trimmed client-side to num_results).
func (s *SerplyTools) SearchNews(p SearchParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON("Please provide a query to search for"), nil
	}
	num := s.num(p.NumResults)
	data, errMsg := s.request("news", p.Query, num)
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type item struct {
		Title     interface{} `json:"title"`
		Link      interface{} `json:"link"`
		Source    interface{} `json:"source"`
		Published interface{} `json:"published"`
	}
	entries := list(data, "entries")
	if len(entries) > num {
		entries = entries[:num]
	}
	results := []item{}
	for _, it := range entries {
		e := obj(it)
		results = append(results, item{get(e, "title"), get(e, "link"), get(obj(get(e, "source")), "title"), get(e, "published")})
	}
	return restkit.JSONString(map[string]interface{}{"news_results": results}), nil
}

// SearchScholar searches Google Scholar.
func (s *SerplyTools) SearchScholar(p SearchParams) (interface{}, error) {
	if p.Query == "" {
		return restkit.ErrorJSON("Please provide a query to search for"), nil
	}
	data, errMsg := s.request("scholar", p.Query, s.num(p.NumResults))
	if errMsg != "" {
		return restkit.ErrorJSON(errMsg), nil
	}
	type item struct {
		Title       interface{} `json:"title"`
		Link        interface{} `json:"link"`
		Description interface{} `json:"description"`
		Authors     interface{} `json:"authors"`
		Citations   interface{} `json:"citations"`
		PDF         interface{} `json:"pdf"`
	}
	results := []item{}
	for _, it := range list(data, "articles") {
		a := obj(it)
		results = append(results, item{
			get(a, "title"), get(a, "link"), get(a, "description"),
			get(obj(get(a, "author")), "names"),
			get(obj(get(obj(get(a, "extras")), "citations")), "count"),
			get(obj(get(a, "doc")), "link"),
		})
	}
	return restkit.JSONString(map[string]interface{}{"scholar_results": results}), nil
}

// Execute implements toolkit.Tool.
func (s *SerplyTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}

// Package searxng ports Agno's Searxng toolkit: web, image, IT, map, music,
// news, science and video search through a self-hosted SearXNG instance's
// JSON API (/search?format=json).
package searxng

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// SearxngTools searches a SearXNG instance (Python: Searxng / SearxngTools).
type SearxngTools struct {
	toolkit.Toolkit
	host            string
	engines         []string
	fixedMaxResults int
	httpClient      *http.Client
	client          *restkit.Client
}

// Option configures SearxngTools.
type Option func(*SearxngTools)

// WithHost sets the SearXNG instance URL (Python's required `host`).
func WithHost(host string) Option { return func(s *SearxngTools) { s.host = host } }

// WithBaseURL is an alias of WithHost.
func WithBaseURL(u string) Option { return WithHost(u) }

// WithEngines restricts the search to the given SearXNG engines.
func WithEngines(engines ...string) Option { return func(s *SearxngTools) { s.engines = engines } }

// WithFixedMaxResults forces the number of results regardless of the call argument.
func WithFixedMaxResults(n int) Option { return func(s *SearxngTools) { s.fixedMaxResults = n } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(s *SearxngTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SearxngTools) { s.httpClient = c } }

// SearchParams are the parameters shared by every search method.
type SearchParams struct {
	Query      string `json:"query" description:"The query to search with." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"The maximum number of results to return. Defaults to 5."`
}

// New creates the SearXNG toolkit. The host can also come from SEARXNG_HOST.
func New(opts ...Option) *SearxngTools {
	s := &SearxngTools{}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "searxng"
	s.Toolkit.Description = "Search the web, images, news, videos and more through a SearXNG instance."
	for _, o := range opts {
		o(s)
	}
	s.host = strings.TrimRight(restkit.Env(s.host, "SEARXNG_HOST"), "/")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{HTTP: s.httpClient, BaseURL: s.host, Name: "searxng"}

	s.Toolkit.Register("SearchWeb", "Use this function to search the web.", s, s.SearchWeb, SearchParams{})
	s.Toolkit.Register("ImageSearch", "Use this function to search for images.", s, s.ImageSearch, SearchParams{})
	s.Toolkit.Register("ItSearch", "Use this function to search for IT related information.", s, s.ItSearch, SearchParams{})
	s.Toolkit.Register("MapSearch", "Use this function to search maps.", s, s.MapSearch, SearchParams{})
	s.Toolkit.Register("MusicSearch", "Use this function to search for information related to music.", s, s.MusicSearch, SearchParams{})
	s.Toolkit.Register("NewsSearch", "Use this function to search for news.", s, s.NewsSearch, SearchParams{})
	s.Toolkit.Register("ScienceSearch", "Use this function to search for information related to science.", s, s.ScienceSearch, SearchParams{})
	s.Toolkit.Register("VideoSearch", "Use this function to search for videos.", s, s.VideoSearch, SearchParams{})
	return s
}

// SearchWeb searches the web (no category).
func (s *SearxngTools) SearchWeb(p SearchParams) (interface{}, error) { return s.search(p, "") }

// ImageSearch searches the "images" category.
func (s *SearxngTools) ImageSearch(p SearchParams) (interface{}, error) { return s.search(p, "images") }

// ItSearch searches the "it" category.
func (s *SearxngTools) ItSearch(p SearchParams) (interface{}, error) { return s.search(p, "it") }

// MapSearch searches the "map" category.
func (s *SearxngTools) MapSearch(p SearchParams) (interface{}, error) { return s.search(p, "map") }

// MusicSearch searches the "music" category.
func (s *SearxngTools) MusicSearch(p SearchParams) (interface{}, error) { return s.search(p, "music") }

// NewsSearch searches the "news" category.
func (s *SearxngTools) NewsSearch(p SearchParams) (interface{}, error) { return s.search(p, "news") }

// ScienceSearch searches the "science" category.
func (s *SearxngTools) ScienceSearch(p SearchParams) (interface{}, error) {
	return s.search(p, "science")
}

// VideoSearch searches the "videos" category.
func (s *SearxngTools) VideoSearch(p SearchParams) (interface{}, error) { return s.search(p, "videos") }

// search mirrors Searxng._search: it returns the instance's JSON response with
// "results" trimmed to max_results, or "Error fetching results from searxng: ...".
func (s *SearxngTools) search(p SearchParams, category string) (interface{}, error) {
	if s.host == "" {
		return nil, fmt.Errorf("searxng host is required (WithHost or SEARXNG_HOST)")
	}
	q := url.Values{"format": {"json"}, "q": {p.Query}}
	if len(s.engines) > 0 {
		q.Set("engines", strings.Join(s.engines, ","))
	}
	if category != "" {
		q.Set("categories", category)
	}
	maxResults := p.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}
	if s.fixedMaxResults > 0 {
		maxResults = s.fixedMaxResults
	}
	var resp map[string]interface{}
	if err := s.client.Into(http.MethodGet, "/search", q, nil, &resp); err != nil {
		return fmt.Sprintf("Error fetching results from searxng: %v", err), nil
	}
	if resp == nil {
		resp = map[string]interface{}{}
	}
	results, _ := resp["results"].([]interface{})
	if len(results) > maxResults {
		results = results[:maxResults]
	}
	if results == nil {
		results = []interface{}{}
	}
	resp["results"] = results
	return restkit.CompactJSON(resp), nil
}

// Execute implements toolkit.Tool.
func (s *SearxngTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}

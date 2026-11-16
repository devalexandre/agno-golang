// Package jina ports Agno's JinaReaderTools: read a URL through the Jina
// Reader API (https://r.jina.ai/) and search the web through Jina Search
// (https://s.jina.ai/).
package jina

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const (
	// DefaultBaseURL is the Jina Reader API URL (the target URL is appended).
	DefaultBaseURL = "https://r.jina.ai/"
	// DefaultSearchURL is the Jina Search API URL.
	DefaultSearchURL = "https://s.jina.ai/"
)

// JinaReaderTools reads URLs and searches the web with Jina.
type JinaReaderTools struct {
	toolkit.Toolkit
	apiKey             string
	baseURL            string
	searchURL          string
	maxContentLength   int
	readerTimeout      int
	searchQueryContent bool
	enableReadURL      bool
	enableSearchQuery  bool
	all                bool
	httpClient         *http.Client
	client             *restkit.Client
}

// Option configures JinaReaderTools.
type Option func(*JinaReaderTools)

// WithAPIKey sets the API key (default: JINA_API_KEY). It is optional.
func WithAPIKey(key string) Option { return func(j *JinaReaderTools) { j.apiKey = key } }

// WithBaseURL overrides the Reader URL (default https://r.jina.ai/).
func WithBaseURL(u string) Option { return func(j *JinaReaderTools) { j.baseURL = u } }

// WithSearchURL overrides the Search URL (default https://s.jina.ai/).
func WithSearchURL(u string) Option { return func(j *JinaReaderTools) { j.searchURL = u } }

// WithMaxContentLength sets the max returned length in characters (default 10000).
func WithMaxContentLength(n int) Option { return func(j *JinaReaderTools) { j.maxContentLength = n } }

// WithTimeout sets the HTTP timeout and, like Python's `timeout`, sends it to
// Jina as the X-Timeout header (whole seconds).
func WithTimeout(d time.Duration) Option {
	return func(j *JinaReaderTools) {
		j.Toolkit.Timeout = d
		j.readerTimeout = int(d / time.Second)
	}
}

// WithSearchQueryContent toggles full page content in search results (default true).
func WithSearchQueryContent(b bool) Option {
	return func(j *JinaReaderTools) { j.searchQueryContent = b }
}

// WithEnableReadURL toggles ReadURL (default true).
func WithEnableReadURL(b bool) Option { return func(j *JinaReaderTools) { j.enableReadURL = b } }

// WithEnableSearchQuery toggles SearchQuery (default false).
func WithEnableSearchQuery(b bool) Option {
	return func(j *JinaReaderTools) { j.enableSearchQuery = b }
}

// WithAll enables every tool.
func WithAll() Option { return func(j *JinaReaderTools) { j.all = true } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(j *JinaReaderTools) { j.httpClient = c } }

// ReadURLParams are the parameters of ReadURL.
type ReadURLParams struct {
	URL string `json:"url" description:"The URL to read." required:"true"`
}

// SearchQueryParams are the parameters of SearchQuery.
type SearchQueryParams struct {
	Query string `json:"query" description:"The search query." required:"true"`
}

// New creates the Jina Reader toolkit.
func New(opts ...Option) *JinaReaderTools {
	j := &JinaReaderTools{
		baseURL: DefaultBaseURL, searchURL: DefaultSearchURL, maxContentLength: 10000,
		searchQueryContent: true, enableReadURL: true,
	}
	j.Toolkit = toolkit.NewToolkit()
	j.Toolkit.Name = "jina_reader_tools"
	j.Toolkit.Description = "Read web pages and search the web using the Jina Reader API."
	for _, o := range opts {
		o(j)
	}
	j.apiKey = restkit.Env(j.apiKey, "JINA_API_KEY")
	if j.httpClient == nil {
		j.httpClient = j.Toolkit.HTTPClient()
	}
	j.client = &restkit.Client{HTTP: j.httpClient, Name: "Jina API"}
	if j.all || j.enableReadURL {
		j.Toolkit.Register("ReadURL", "Reads a URL and returns the truncated content using Jina Reader API.", j, j.ReadURL, ReadURLParams{})
	}
	if j.all || j.enableSearchQuery {
		j.Toolkit.Register("SearchQuery", "Performs a web search using Jina Reader API and returns the truncated results.", j, j.SearchQuery, SearchQueryParams{})
	}
	return j
}

func (j *JinaReaderTools) headers() map[string]string {
	h := map[string]string{
		"Accept":                "application/json",
		"X-With-Links-Summary":  "true",
		"X-With-Images-Summary": "true",
	}
	if j.apiKey != "" {
		h["Authorization"] = "Bearer " + j.apiKey
	}
	if j.readerTimeout > 0 {
		h["X-Timeout"] = strconv.Itoa(j.readerTimeout)
	}
	return h
}

func (j *JinaReaderTools) truncate(content string) string {
	if j.maxContentLength > 0 && len([]rune(content)) > j.maxContentLength {
		return restkit.Truncate(content, j.maxContentLength) + "... (content truncated)"
	}
	return content
}

// ReadURL fetches base_url+url and returns the (truncated) JSON content.
func (j *JinaReaderTools) ReadURL(p ReadURLParams) (interface{}, error) {
	c := *j.client
	c.Headers = j.headers()
	resp, err := c.JSON(http.MethodGet, j.baseURL+p.URL, nil, nil)
	if err != nil {
		return fmt.Sprintf("Error reading URL: %v", err), nil
	}
	return j.truncate(restkit.CompactJSON(resp)), nil
}

// SearchQuery posts {"q": query} to the search URL and returns the (truncated) JSON content.
func (j *JinaReaderTools) SearchQuery(p SearchQueryParams) (interface{}, error) {
	c := *j.client
	c.Headers = j.headers()
	if !j.searchQueryContent {
		c.Headers["X-Respond-With"] = "no-content"
	}
	resp, err := c.JSON(http.MethodPost, j.searchURL, nil, map[string]string{"q": p.Query})
	if err != nil {
		return fmt.Sprintf("Error performing search: %v", err), nil
	}
	return j.truncate(restkit.CompactJSON(resp)), nil
}

// Execute implements toolkit.Tool.
func (j *JinaReaderTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return j.Toolkit.Execute(methodName, input)
}

// Package youcom ports Agno's YouTools: web search through the You.com Search
// API (GET https://ydc-index.io/v1/search).
package youcom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the You.com Search API base URL.
const DefaultBaseURL = "https://ydc-index.io"

// YouTools searches the web with You.com.
type YouTools struct {
	toolkit.Toolkit
	apiKey          string
	baseURL         string
	numResults      int
	livecrawl       string
	livecrawlFmts   []string
	textLengthLimit int
	includeDomains  []string
	excludeDomains  []string
	country         string
	freshness       string
	language        string
	safesearch      string
	offset          *int
	boostDomains    []string
	crawlTimeout    int
	searchParams    map[string]interface{}
	format          string
	configErr       error
	httpClient      *http.Client
	client          *restkit.Client
}

// Option configures YouTools.
type Option func(*YouTools)

// WithAPIKey sets the API key (default: YDC_API_KEY).
func WithAPIKey(key string) Option { return func(y *YouTools) { y.apiKey = key } }

// WithBaseURL overrides the API base URL (default: YDC_BASE_URL, then https://ydc-index.io).
func WithBaseURL(u string) Option { return func(y *YouTools) { y.baseURL = u } }

// WithNumResults sets the default number of results (default 5).
func WithNumResults(n int) Option { return func(y *YouTools) { y.numResults = n } }

// WithLivecrawl enables live crawling ("web", "news" or "all").
func WithLivecrawl(mode string) Option { return func(y *YouTools) { y.livecrawl = mode } }

// WithLivecrawlFormats sets the livecrawl content formats (default "markdown").
func WithLivecrawlFormats(f ...string) Option { return func(y *YouTools) { y.livecrawlFmts = f } }

// WithTextLengthLimit sets the max text length per result (default 1000, 0 = unlimited).
func WithTextLengthLimit(n int) Option { return func(y *YouTools) { y.textLengthLimit = n } }

// WithIncludeDomains restricts results to these domains.
func WithIncludeDomains(d ...string) Option { return func(y *YouTools) { y.includeDomains = d } }

// WithExcludeDomains excludes results from these domains.
func WithExcludeDomains(d ...string) Option { return func(y *YouTools) { y.excludeDomains = d } }

// WithBoostDomains boosts these domains in ranking.
func WithBoostDomains(d ...string) Option { return func(y *YouTools) { y.boostDomains = d } }

// WithCountry sets the geographical focus of the results.
func WithCountry(c string) Option { return func(y *YouTools) { y.country = c } }

// WithFreshness sets the result freshness ("day", "week", "month", "year").
func WithFreshness(f string) Option { return func(y *YouTools) { y.freshness = f } }

// WithLanguage sets the result language (BCP 47).
func WithLanguage(l string) Option { return func(y *YouTools) { y.language = l } }

// WithSafesearch sets the safesearch filter ("off", "moderate", "strict").
func WithSafesearch(s string) Option { return func(y *YouTools) { y.safesearch = s } }

// WithOffset sets the pagination offset (0-9).
func WithOffset(o int) Option { return func(y *YouTools) { y.offset = &o } }

// WithCrawlTimeout sets the livecrawl timeout in seconds (1-60, default 10).
func WithCrawlTimeout(s int) Option { return func(y *YouTools) { y.crawlTimeout = s } }

// WithSearchParams merges extra query parameters into every request.
func WithSearchParams(p map[string]interface{}) Option {
	return func(y *YouTools) { y.searchParams = p }
}

// WithFormat sets the output format: "json" (default) or "markdown".
func WithFormat(f string) Option { return func(y *YouTools) { y.format = f } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(y *YouTools) { y.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(y *YouTools) { y.httpClient = c } }

// YouSearchParams are the parameters of YouSearch.
type YouSearchParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Override the configured result count."`
}

// New creates the You.com toolkit. Invalid configuration (crawl_timeout or
// offset out of range, include_domains combined with exclude/boost domains)
// is reported when YouSearch is called.
func New(opts ...Option) *YouTools {
	y := &YouTools{numResults: 5, livecrawlFmts: []string{"markdown"}, textLengthLimit: 1000, crawlTimeout: 10, format: "json"}
	y.Toolkit = toolkit.NewToolkit()
	y.Toolkit.Name = "youcom"
	y.Toolkit.Description = "Search the web using the You.com Search API."
	for _, o := range opts {
		o(y)
	}
	y.apiKey = restkit.Env(y.apiKey, "YDC_API_KEY")
	y.baseURL = strings.TrimRight(restkit.Env(y.baseURL, "YDC_BASE_URL"), "/")
	if y.baseURL == "" {
		y.baseURL = DefaultBaseURL
	}
	switch {
	case y.crawlTimeout < 1 || y.crawlTimeout > 60:
		y.configErr = fmt.Errorf("crawl_timeout must be between 1 and 60 seconds")
	case y.offset != nil && (*y.offset < 0 || *y.offset > 9):
		y.configErr = fmt.Errorf("offset must be between 0 and 9")
	case len(y.includeDomains) > 0 && (len(y.excludeDomains) > 0 || len(y.boostDomains) > 0):
		y.configErr = fmt.Errorf("include_domains cannot be combined with exclude_domains or boost_domains")
	}
	if y.httpClient == nil {
		y.httpClient = y.Toolkit.HTTPClient()
	}
	y.client = &restkit.Client{
		HTTP:    y.httpClient,
		BaseURL: y.baseURL,
		Name:    "You.com API",
		Headers: map[string]string{"X-API-Key": y.apiKey},
	}
	y.Toolkit.Register("YouSearch", "Search the web using the You.com Search API.", y, y.YouSearch, YouSearchParams{})
	return y
}

type result struct {
	URL           interface{} `json:"url,omitempty"`
	Title         interface{} `json:"title,omitempty"`
	Snippet       interface{} `json:"snippet,omitempty"`
	Text          string      `json:"text,omitempty"`
	PublishedDate interface{} `json:"published_date,omitempty"`
}

// YouSearch searches You.com and returns the cleaned results as JSON (indent 4)
// or markdown, or "Error: ..." on failure.
func (y *YouTools) YouSearch(p YouSearchParams) (interface{}, error) {
	if y.configErr != nil {
		return nil, y.configErr
	}
	if y.apiKey == "" {
		return nil, fmt.Errorf("YDC_API_KEY is required")
	}
	count := p.NumResults
	if count <= 0 {
		count = y.numResults
	}
	q := url.Values{"query": {p.Query}, "count": {strconv.Itoa(count)}}
	if y.livecrawl != "" {
		q.Set("livecrawl", y.livecrawl)
		q.Set("crawl_timeout", strconv.Itoa(y.crawlTimeout))
		for _, f := range y.livecrawlFmts {
			for _, part := range strings.Split(f, ",") {
				if part = strings.TrimSpace(part); part != "" {
					q.Add("livecrawl_formats", part)
				}
			}
		}
	}
	setList := func(k string, v []string) {
		if len(v) > 0 {
			q.Set(k, strings.Join(v, ","))
		}
	}
	setList("include_domains", y.includeDomains)
	setList("exclude_domains", y.excludeDomains)
	setList("boost_domains", y.boostDomains)
	for k, v := range map[string]string{"country": y.country, "freshness": y.freshness, "language": y.language, "safesearch": y.safesearch} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if y.offset != nil {
		q.Set("offset", strconv.Itoa(*y.offset))
	}
	for k, v := range y.searchParams {
		q.Del(k)
		if arr, ok := v.([]string); ok {
			for _, s := range arr {
				q.Add(k, s)
			}
			continue
		}
		q.Set(k, fmt.Sprint(v))
	}

	var data map[string]interface{}
	if err := y.client.Into(http.MethodGet, "/v1/search", q, nil, &data); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return y.formatResults(p.Query, data), nil
}

func truthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case float64:
		return t != 0
	case []interface{}:
		return len(t) > 0
	case map[string]interface{}:
		return len(t) > 0
	}
	return true
}

func firstTruthy(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if truthy(m[k]) {
			return m[k]
		}
	}
	return nil
}

func (y *YouTools) formatResults(query string, data map[string]interface{}) string {
	var raw []interface{}
	switch r := data["results"].(type) {
	case map[string]interface{}:
		// Results are grouped by source (web/news); flatten them.
		for _, sec := range []string{"web", "news"} {
			if arr, ok := r[sec].([]interface{}); ok {
				raw = append(raw, arr...)
			}
		}
		for k, sec := range r {
			if k == "web" || k == "news" {
				continue
			}
			if arr, ok := sec.([]interface{}); ok {
				raw = append(raw, arr...)
			}
		}
	case []interface{}:
		raw = r
	}
	if len(raw) == 0 {
		raw, _ = data["hits"].([]interface{})
	}
	cleaned := []result{}
	for _, it := range raw {
		r, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		var e result
		if truthy(r["url"]) {
			e.URL = r["url"]
		}
		if truthy(r["title"]) {
			e.Title = r["title"]
		}
		e.Snippet = firstTruthy(r, "description", "snippet")
		contents, ok := r["contents"].(map[string]interface{})
		if !ok {
			contents = r
		}
		text := ""
		if v := firstTruthy(contents, "markdown", "text", "html", "content"); v != nil {
			text = fmt.Sprint(v)
		}
		if text == "" {
			if arr, ok := r["snippets"].([]interface{}); ok {
				var parts []string
				for _, s := range arr {
					if str, ok := s.(string); ok {
						parts = append(parts, str)
					}
				}
				text = strings.Join(parts, "\n")
			}
		}
		e.Text = restkit.Truncate(text, y.textLengthLimit)
		e.PublishedDate = firstTruthy(r, "published_date", "page_age")
		cleaned = append(cleaned, e)
	}

	if y.format == "markdown" {
		lines := []string{fmt.Sprintf("# Results for: %s\n", query)}
		for _, r := range cleaned {
			title := r.Title
			if title == nil {
				title = r.URL
			}
			if title == nil {
				title = "Untitled"
			}
			u := ""
			if r.URL != nil {
				u = fmt.Sprint(r.URL)
			}
			lines = append(lines, fmt.Sprintf("## [%v](%s)", title, u))
			if r.Snippet != nil {
				lines = append(lines, fmt.Sprint(r.Snippet))
			}
			if r.Text != "" {
				lines = append(lines, r.Text)
			}
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}
	return dumps(cleaned)
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
func (y *YouTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return y.Toolkit.Execute(methodName, input)
}

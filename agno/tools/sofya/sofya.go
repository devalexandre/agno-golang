// Package sofya ports Agno's SofyaTools: web search with extracted page
// content, URL fetching as markdown and cited multi-source research through
// the Sofya API (https://sofya.co).
package sofya

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Sofya API base URL.
const DefaultBaseURL = "https://sofya.co"

// FetchMaxURLs is Sofya's documented per-call URL limit for fetch.
const FetchMaxURLs = 10

// SofyaTools searches, fetches and researches the web with Sofya.
type SofyaTools struct {
	toolkit.Toolkit
	apiKey         string
	baseURL        string
	httpClient     *http.Client
	client         *restkit.Client
	enableSearch   bool
	enableExtract  bool
	enableResearch bool
	all            bool
	maxResults     int
	searchDepth    string
	includeAnswer  bool
	format         string
}

// Option configures SofyaTools.
type Option func(*SofyaTools)

// WithAPIKey sets the API key (default: SOFYA_API_KEY).
func WithAPIKey(key string) Option { return func(s *SofyaTools) { s.apiKey = key } }

// WithBaseURL overrides the API base URL (default: SOFYA_BASE_URL or https://sofya.co).
func WithBaseURL(u string) Option { return func(s *SofyaTools) { s.baseURL = u } }

// WithTimeout sets the HTTP timeout (default 180s).
func WithTimeout(d time.Duration) Option { return func(s *SofyaTools) { s.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(s *SofyaTools) { s.httpClient = c } }

// WithEnableSearch toggles SearchWeb (default true).
func WithEnableSearch(v bool) Option { return func(s *SofyaTools) { s.enableSearch = v } }

// WithEnableExtract toggles ExtractURLContent (default false).
func WithEnableExtract(v bool) Option { return func(s *SofyaTools) { s.enableExtract = v } }

// WithEnableResearch toggles Research (default false).
func WithEnableResearch(v bool) Option { return func(s *SofyaTools) { s.enableResearch = v } }

// WithAll enables every tool.
func WithAll() Option { return func(s *SofyaTools) { s.all = true } }

// WithMaxResults sets the default number of search results, 1-20 (default 5).
func WithMaxResults(n int) Option { return func(s *SofyaTools) { s.maxResults = n } }

// WithSearchDepth sets the search depth: "snippets" or "basic" (default).
func WithSearchDepth(d string) Option { return func(s *SofyaTools) { s.searchDepth = d } }

// WithIncludeAnswer toggles the AI-generated answer in search results (default true).
func WithIncludeAnswer(v bool) Option { return func(s *SofyaTools) { s.includeAnswer = v } }

// WithFormat sets the output format of search and research: "markdown" (default) or "json".
func WithFormat(f string) Option { return func(s *SofyaTools) { s.format = f } }

// SearchWebParams are the parameters of SearchWeb.
type SearchWebParams struct {
	Query      string `json:"query" description:"Query to search for." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return. Defaults to the configured value."`
}

// ExtractParams are the parameters of ExtractURLContent.
type ExtractParams struct {
	URLs string `json:"urls" description:"Single URL or multiple comma-separated URLs to fetch. Example: \"https://example.com\" or \"https://example.com,https://another.com\"" required:"true"`
}

// ResearchParams are the parameters of Research.
type ResearchParams struct {
	Query string `json:"query" description:"The research question." required:"true"`
}

// New creates the Sofya toolkit.
func New(opts ...Option) *SofyaTools {
	s := &SofyaTools{
		enableSearch:  true,
		maxResults:    5,
		searchDepth:   "basic",
		includeAnswer: true,
		format:        "markdown",
	}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "sofya_tools"
	s.Toolkit.Description = "Search the web, fetch URLs as markdown and run cited research with Sofya."
	s.Toolkit.Timeout = 180 * time.Second
	for _, o := range opts {
		o(s)
	}
	s.apiKey = restkit.Env(s.apiKey, "SOFYA_API_KEY")
	s.baseURL = restkit.Env(s.baseURL, "SOFYA_BASE_URL")
	if s.baseURL == "" {
		s.baseURL = DefaultBaseURL
	}
	s.baseURL = strings.TrimRight(s.baseURL, "/")
	if s.httpClient == nil {
		s.httpClient = s.Toolkit.HTTPClient()
	}
	s.client = &restkit.Client{
		HTTP:    s.httpClient,
		BaseURL: s.baseURL + "/v1",
		Name:    "Sofya API",
		Headers: map[string]string{"Authorization": "Bearer " + s.apiKey, "User-Agent": "agno-sofya"},
	}
	tk := &s.Toolkit
	if s.all || s.enableSearch {
		tk.Register("SearchWeb", "Search the web with Sofya and get extracted page content, not just snippets.", s, s.SearchWeb, SearchWebParams{})
	}
	if s.all || s.enableExtract {
		tk.Register("ExtractURLContent", "Fetch one or more URLs (comma-separated, max 10) as clean markdown using Sofya. Also handles PDF, DOCX, and more.", s, s.ExtractURLContent, ExtractParams{})
	}
	if s.all || s.enableResearch {
		tk.Register("Research", "Run multi-source deep research with Sofya and get back a cited report. Use this for open-ended questions that need several sources, not for a single lookup (use search_web for that).", s, s.Research, ResearchParams{})
	}
	return s
}

type obj = map[string]interface{}

func errJSON(msg string) string { return restkit.CompactJSON(obj{"error": msg}) }

func (s *SofyaTools) post(path string, payload obj) (obj, error) {
	var out obj
	if err := s.client.Into(http.MethodPost, path, nil, payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SearchWeb searches the web and returns markdown (or JSON) results.
func (s *SofyaTools) SearchWeb(params SearchWebParams) (interface{}, error) {
	if s.apiKey == "" {
		return errJSON("Please provide a Sofya API key"), nil
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return errJSON("Please provide a query to search for"), nil
	}
	n := params.MaxResults
	if n == 0 {
		n = s.maxResults
	}
	if n > 20 {
		n = 20
	}
	if n < 1 {
		n = 1
	}
	data, err := s.post("search", obj{
		"query":          query,
		"max_results":    n,
		"search_depth":   s.searchDepth,
		"include_answer": s.includeAnswer,
	})
	if err != nil {
		return errJSON(err.Error()), nil
	}
	type result struct {
		Title         interface{} `json:"title"`
		URL           interface{} `json:"url"`
		Content       interface{} `json:"content"`
		PublishedDate interface{} `json:"published_date"`
	}
	results := []result{}
	for _, r := range restkit.Slice(data, "results") {
		results = append(results, result{r["title"], r["url"], r["content"], r["published_date"]})
	}
	answer := restkit.Str(data, "answer")
	if s.format == "json" {
		clean := obj{"query": params.Query, "results": results}
		if answer != "" {
			clean["answer"] = data["answer"]
		}
		return restkit.CompactJSON(clean), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", params.Query)
	if answer != "" {
		fmt.Fprintf(&b, "### Summary\n%s\n\n", answer)
	}
	for _, r := range results {
		u := str(r.URL)
		title := str(r.Title)
		if title == "" {
			title = u
		}
		if title == "" {
			title = "Untitled"
		}
		fmt.Fprintf(&b, "### [%s](%s)\n%s\n\n", title, u, str(r.Content))
	}
	return b.String(), nil
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// ExtractURLContent fetches URLs as markdown, one section per URL.
func (s *SofyaTools) ExtractURLContent(params ExtractParams) (interface{}, error) {
	if s.apiKey == "" {
		return errJSON("Please provide a Sofya API key"), nil
	}
	var urls []string
	for _, u := range strings.Split(params.URLs, ",") {
		if u = strings.TrimSpace(u); u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return errJSON("No valid URLs provided"), nil
	}
	if len(urls) > FetchMaxURLs {
		urls = urls[:FetchMaxURLs]
	}
	data, err := s.post("fetch", obj{"urls": urls})
	if err != nil {
		return errJSON(err.Error()), nil
	}
	results := restkit.Slice(data, "results")
	if len(results) == 0 {
		return "No content could be extracted from the provided URL(s).", nil
	}
	var b strings.Builder
	for _, r := range results {
		u := restkit.Str(r, "url")
		if _, ok := r["url"]; !ok {
			u = "Unknown URL"
		}
		success := true
		if v, ok := r["success"].(bool); ok {
			success = v
		}
		switch {
		case !success:
			e := restkit.Str(r, "error")
			if _, ok := r["error"]; !ok {
				e = "unknown error"
			}
			fmt.Fprintf(&b, "## %s\n\n**Extraction failed**: %s\n\n", u, e)
		case restkit.Str(r, "content") != "":
			fmt.Fprintf(&b, "## %s\n\n%s\n\n", u, restkit.Str(r, "content"))
		default:
			fmt.Fprintf(&b, "## %s\n\n*No content available*\n\n", u)
		}
	}
	return b.String(), nil
}

// Research runs multi-source research and returns a cited report.
func (s *SofyaTools) Research(params ResearchParams) (interface{}, error) {
	if s.apiKey == "" {
		return errJSON("Please provide a Sofya API key"), nil
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return errJSON("Please provide a query to research"), nil
	}
	data, err := s.post("research", obj{"query": query})
	if err != nil {
		return errJSON(err.Error()), nil
	}
	if s.format == "json" {
		return restkit.CompactJSON(data), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", params.Query, restkit.Str(data, "report"))
	if sources, ok := data["sources"].([]interface{}); ok && len(sources) > 0 {
		b.WriteString("## Sources\n")
		for _, src := range sources {
			if m, ok := src.(map[string]interface{}); ok {
				title := m["title"]
				if _, has := m["title"]; !has {
					title = m["url"]
				}
				fmt.Fprintf(&b, "- [%s](%s)\n", str(title), str(m["url"]))
			} else {
				fmt.Fprintf(&b, "- %s\n", str(src))
			}
		}
	}
	return b.String(), nil
}

// Execute implements toolkit.Tool.
func (s *SofyaTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return s.Toolkit.Execute(methodName, input)
}

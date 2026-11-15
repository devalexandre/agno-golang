// Package valyu ports Agno's ValyuTools: academic, web and within-paper search
// through the Valyu DeepSearch API (POST https://api.valyu.ai/v1/deepsearch),
// the REST endpoint wrapped by the valyu Python SDK.
package valyu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Valyu API base URL.
const DefaultBaseURL = "https://api.valyu.ai/v1"

// academicSources are the proprietary sources used by SearchAcademicSources.
var academicSources = []string{"valyu/valyu-arxiv", "valyu/valyu-pubmed", "wiley/wiley-finance-papers", "wiley/wiley-finance-books"}

// ValyuTools searches academic and web sources with Valyu.
type ValyuTools struct {
	toolkit.Toolkit
	apiKey             string
	baseURL            string
	enableAcademic     bool
	enableWeb          bool
	enablePaper        bool
	all                bool
	textLength         int
	maxResults         int
	relevanceThreshold float64
	contentCategory    string
	searchStartDate    string
	searchEndDate      string
	searchDomains      []string
	sources            []string
	maxPrice           float64
	toolCallMode       bool
	httpClient         *http.Client
	client             *restkit.Client
}

// Option configures ValyuTools.
type Option func(*ValyuTools)

// WithAPIKey sets the API key (default: VALYU_API_KEY).
func WithAPIKey(key string) Option { return func(v *ValyuTools) { v.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(v *ValyuTools) { v.baseURL = u } }

// WithEnableAcademicSearch toggles SearchAcademicSources (default true).
func WithEnableAcademicSearch(b bool) Option { return func(v *ValyuTools) { v.enableAcademic = b } }

// WithEnableWebSearch toggles SearchWeb (default true).
func WithEnableWebSearch(b bool) Option { return func(v *ValyuTools) { v.enableWeb = b } }

// WithEnablePaperSearch toggles SearchWithinPaper (default true).
func WithEnablePaperSearch(b bool) Option { return func(v *ValyuTools) { v.enablePaper = b } }

// WithAll enables every tool.
func WithAll() Option { return func(v *ValyuTools) { v.all = true } }

// WithTextLength sets the max content length per result (default 1000).
func WithTextLength(n int) Option { return func(v *ValyuTools) { v.textLength = n } }

// WithMaxResults sets the max number of results (default 10).
func WithMaxResults(n int) Option { return func(v *ValyuTools) { v.maxResults = n } }

// WithRelevanceThreshold sets the minimum relevance score (default 0.5).
func WithRelevanceThreshold(f float64) Option {
	return func(v *ValyuTools) { v.relevanceThreshold = f }
}

// WithContentCategory sets the default content category.
func WithContentCategory(c string) Option { return func(v *ValyuTools) { v.contentCategory = c } }

// WithSearchDates sets the default start/end dates (YYYY-MM-DD).
func WithSearchDates(start, end string) Option {
	return func(v *ValyuTools) { v.searchStartDate, v.searchEndDate = start, end }
}

// WithSearchDomains stores the domains to search within (kept for parity; Python does not send it).
func WithSearchDomains(d ...string) Option { return func(v *ValyuTools) { v.searchDomains = d } }

// WithSources sets the default included sources.
func WithSources(s ...string) Option { return func(v *ValyuTools) { v.sources = s } }

// WithMaxPrice sets the max price per call (default 30.0).
func WithMaxPrice(f float64) Option { return func(v *ValyuTools) { v.maxPrice = f } }

// WithToolCallMode sets is_tool_call (default false).
func WithToolCallMode(b bool) Option { return func(v *ValyuTools) { v.toolCallMode = b } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(v *ValyuTools) { v.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(v *ValyuTools) { v.httpClient = c } }

// AcademicParams are the parameters of SearchAcademicSources.
type AcademicParams struct {
	Query     string `json:"query" description:"Research question or topic" required:"true"`
	StartDate string `json:"start_date,omitempty" description:"Filter papers after this date (YYYY-MM-DD)"`
	EndDate   string `json:"end_date,omitempty" description:"Filter papers before this date (YYYY-MM-DD)"`
}

// WebParams are the parameters of SearchWeb.
type WebParams struct {
	Query           string `json:"query" description:"Search query" required:"true"`
	StartDate       string `json:"start_date,omitempty" description:"Filter content after this date (YYYY-MM-DD)"`
	EndDate         string `json:"end_date,omitempty" description:"Filter content before this date (YYYY-MM-DD)"`
	ContentCategory string `json:"content_category,omitempty" description:"Description of the category of the query"`
}

// PaperParams are the parameters of SearchWithinPaper.
type PaperParams struct {
	PaperURL string `json:"paper_url" description:"ArXiv paper URL" required:"true"`
	Query    string `json:"query" description:"Search query" required:"true"`
}

// New creates the Valyu toolkit.
func New(opts ...Option) *ValyuTools {
	v := &ValyuTools{
		baseURL: DefaultBaseURL, enableAcademic: true, enableWeb: true, enablePaper: true,
		textLength: 1000, maxResults: 10, relevanceThreshold: 0.5, maxPrice: 30.0,
	}
	v.Toolkit = toolkit.NewToolkit()
	v.Toolkit.Name = "valyu_search"
	v.Toolkit.Description = "Academic and web search using the Valyu API."
	for _, o := range opts {
		o(v)
	}
	v.apiKey = restkit.Env(v.apiKey, "VALYU_API_KEY")
	if v.httpClient == nil {
		v.httpClient = v.Toolkit.HTTPClient()
	}
	v.client = &restkit.Client{
		HTTP:    v.httpClient,
		BaseURL: v.baseURL,
		Name:    "Valyu API",
		Headers: map[string]string{"x-api-key": v.apiKey},
	}
	if v.all || v.enableAcademic {
		v.Toolkit.Register("SearchAcademicSources", "Search academic sources (ArXiv, PubMed, academic publishers). Returns a JSON array of academic papers.", v, v.SearchAcademicSources, AcademicParams{})
	}
	if v.all || v.enableWeb {
		v.Toolkit.Register("SearchWeb", "Search web sources for real-time information. Returns a JSON array of web search results.", v, v.SearchWeb, WebParams{})
	}
	if v.all || v.enablePaper {
		v.Toolkit.Register("SearchWithinPaper", "Search within a specific ArXiv paper. Returns a JSON array of relevant sections from the paper.", v, v.SearchWithinPaper, PaperParams{})
	}
	return v
}

// SearchAcademicSources searches ArXiv, PubMed and Wiley finance sources.
func (v *ValyuTools) SearchAcademicSources(p AcademicParams) (interface{}, error) {
	return v.search(p.Query, "proprietary", "", academicSources, p.StartDate, p.EndDate)
}

// SearchWeb searches web sources.
func (v *ValyuTools) SearchWeb(p WebParams) (interface{}, error) {
	return v.search(p.Query, "web", p.ContentCategory, nil, p.StartDate, p.EndDate)
}

// SearchWithinPaper searches within a specific paper URL.
func (v *ValyuTools) SearchWithinPaper(p PaperParams) (interface{}, error) {
	if !strings.HasPrefix(p.PaperURL, "https:/") {
		return "Error: Invalid paper URL format", nil
	}
	return v.search(p.Query, "proprietary", "", []string{p.PaperURL}, "", "")
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (v *ValyuTools) search(query, searchType, category string, sources []string, start, end string) (interface{}, error) {
	if v.apiKey == "" {
		return nil, fmt.Errorf("VALYU_API_KEY not set. Please set the VALYU_API_KEY environment variable")
	}
	body := map[string]interface{}{
		"query":               query,
		"search_type":         searchType,
		"max_num_results":     v.maxResults,
		"is_tool_call":        v.toolCallMode,
		"relevance_threshold": v.relevanceThreshold,
		"max_price":           v.maxPrice,
	}
	if len(sources) == 0 {
		sources = v.sources
	}
	if len(sources) > 0 {
		body["included_sources"] = sources
	}
	if c := or(category, v.contentCategory); c != "" {
		body["category"] = c
	}
	if s := or(start, v.searchStartDate); s != "" {
		body["start_date"] = s
	}
	if e := or(end, v.searchEndDate); e != "" {
		body["end_date"] = e
	}

	var resp struct {
		Success *bool                    `json:"success"`
		Error   string                   `json:"error"`
		Results []map[string]interface{} `json:"results"`
	}
	_, data, err := v.client.Raw(http.MethodPost, "/deepsearch", nil, body)
	if len(data) > 0 {
		_ = json.Unmarshal(data, &resp)
	}
	if err != nil {
		var apiErr *restkit.APIError
		if errors.As(err, &apiErr) && resp.Error != "" {
			return "Error: " + resp.Error, nil
		}
		return fmt.Sprintf("Error: Valyu search failed: %v", err), nil
	}
	if resp.Success != nil && !*resp.Success {
		return "Error: " + or(resp.Error, "Search request failed"), nil
	}
	return v.parseResults(resp.Results), nil
}

type parsedResult struct {
	URL            interface{} `json:"url,omitempty"`
	Title          interface{} `json:"title,omitempty"`
	Source         interface{} `json:"source,omitempty"`
	RelevanceScore interface{} `json:"relevance_score"`
	Content        interface{} `json:"content,omitempty"`
	Description    interface{} `json:"description,omitempty"`
}

func nonEmpty(v interface{}) interface{} {
	if s, ok := v.(string); ok && s == "" {
		return nil
	}
	return v
}

func (v *ValyuTools) parseResults(results []map[string]interface{}) string {
	parsed := []parsedResult{}
	for _, r := range results {
		pr := parsedResult{
			URL:            nonEmpty(r["url"]),
			Title:          nonEmpty(r["title"]),
			Source:         nonEmpty(r["source"]),
			RelevanceScore: r["relevance_score"],
			Description:    nonEmpty(r["description"]),
		}
		content := nonEmpty(r["content"])
		if s, ok := content.(string); ok && v.textLength > 0 && len([]rune(s)) > v.textLength {
			content = restkit.Truncate(s, v.textLength) + "..."
		}
		pr.Content = content
		parsed = append(parsed, pr)
	}
	return restkit.JSONString(parsed)
}

// Execute implements toolkit.Tool.
func (v *ValyuTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return v.Toolkit.Execute(methodName, input)
}

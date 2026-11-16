// Package agentql ports Agno's AgentQLTools: scraping web pages with AgentQL
// queries. The Python toolkit drives a local Playwright browser; this port
// uses AgentQL's hosted REST API (POST https://api.agentql.com/v1/query-data),
// which runs the same queries in AgentQL's browser.
package agentql

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the AgentQL REST API base URL.
const DefaultBaseURL = "https://api.agentql.com/v1"

// DefaultTimeout bounds a query (the remote browser loads the page first).
const DefaultTimeout = 120 * time.Second

// textSearchQuery is the query used by ScrapeWebsite.
const textSearchQuery = `
{
    text_content[]
}
`

// AgentQLTools scrapes websites with AgentQL.
type AgentQLTools struct {
	toolkit.Toolkit
	apiKey             string
	baseURL            string
	agentqlQuery       string
	enableScrape       bool
	enableCustomScrape bool
	all                bool
	mode               string
	httpClient         *http.Client
	client             *restkit.Client
}

// Option configures AgentQLTools.
type Option func(*AgentQLTools)

// WithAPIKey sets the API key (default: AGENTQL_API_KEY).
func WithAPIKey(key string) Option { return func(a *AgentQLTools) { a.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(a *AgentQLTools) { a.baseURL = u } }

// WithTimeout sets the HTTP timeout (default 120s).
func WithTimeout(d time.Duration) Option { return func(a *AgentQLTools) { a.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(a *AgentQLTools) { a.httpClient = c } }

// WithEnableScrapeWebsite toggles ScrapeWebsite (default true).
func WithEnableScrapeWebsite(v bool) Option { return func(a *AgentQLTools) { a.enableScrape = v } }

// WithEnableCustomScrapeWebsite toggles CustomScrapeWebsite (default false).
// As in Python it is only registered when a custom query is set.
func WithEnableCustomScrapeWebsite(v bool) Option {
	return func(a *AgentQLTools) { a.enableCustomScrape = v }
}

// WithAll enables every tool.
func WithAll() Option { return func(a *AgentQLTools) { a.all = true } }

// WithAgentQLQuery sets the custom AgentQL query used by CustomScrapeWebsite.
func WithAgentQLQuery(q string) Option { return func(a *AgentQLTools) { a.agentqlQuery = q } }

// WithMode sets the AgentQL REST query mode: "fast" (API default) or "standard".
func WithMode(m string) Option { return func(a *AgentQLTools) { a.mode = m } }

// URLParams carry the page URL.
type URLParams struct {
	URL string `json:"url" description:"The URL of the website to scrape" required:"true"`
}

// New creates the AgentQL toolkit.
func New(opts ...Option) *AgentQLTools {
	a := &AgentQLTools{baseURL: DefaultBaseURL, enableScrape: true}
	a.Toolkit = toolkit.NewToolkit()
	a.Toolkit.Name = "agentql_tools"
	a.Toolkit.Description = "Scrape websites using AgentQL queries."
	a.Toolkit.Timeout = DefaultTimeout
	for _, o := range opts {
		o(a)
	}
	a.apiKey = restkit.Env(a.apiKey, "AGENTQL_API_KEY")
	if a.httpClient == nil {
		a.httpClient = a.Toolkit.HTTPClient()
	}
	a.client = &restkit.Client{
		HTTP:    a.httpClient,
		BaseURL: a.baseURL,
		Name:    "AgentQL API",
		Headers: map[string]string{"X-API-Key": a.apiKey},
	}
	if a.all || a.enableScrape {
		a.Toolkit.Register("ScrapeWebsite", "Scrape all text content from a website using AgentQL.", a, a.ScrapeWebsite, URLParams{})
	}
	// Python registers the custom tool whenever a custom query is provided.
	if a.agentqlQuery != "" {
		a.Toolkit.Register("CustomScrapeWebsite", "Scrape a website using a custom AgentQL query.", a, a.CustomScrapeWebsite, URLParams{})
	}
	return a
}

func (a *AgentQLTools) queryData(pageURL, query string) (interface{}, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("AGENTQL_API_KEY not set. Please set the AGENTQL_API_KEY environment variable")
	}
	body := map[string]interface{}{"url": pageURL, "query": query}
	if a.mode != "" {
		body["params"] = map[string]interface{}{"mode": a.mode}
	}
	var resp struct {
		Data interface{} `json:"data"`
	}
	if err := a.client.Into(http.MethodPost, "/query-data", nil, body, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ScrapeWebsite extracts all text content from a page.
func (a *AgentQLTools) ScrapeWebsite(params URLParams) (interface{}, error) {
	if params.URL == "" {
		return "No URL provided", nil
	}
	data, err := a.queryData(params.URL, textSearchQuery)
	if err != nil {
		return fmt.Sprintf("Error extracting text: %v", err), nil
	}
	m, ok := data.(map[string]interface{})
	if !ok {
		return "No text content found", nil
	}
	items, ok := m["text_content"].([]interface{})
	if !ok {
		return "No text content found", nil
	}
	seen := map[string]bool{}
	var out []string
	for _, it := range items {
		s, _ := it.(string)
		if strings.TrimSpace(s) == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return strings.Join(out, " "), nil
}

// CustomScrapeWebsite runs the configured custom AgentQL query on a page and
// returns the extracted data as JSON.
func (a *AgentQLTools) CustomScrapeWebsite(params URLParams) (interface{}, error) {
	if params.URL == "" {
		return "No URL provided", nil
	}
	if a.agentqlQuery == "" {
		return "Custom AgentQL query not provided. Please provide a custom AgentQL query.", nil
	}
	data, err := a.queryData(params.URL, a.agentqlQuery)
	if err != nil {
		return fmt.Sprintf("Error extracting text: %v", err), nil
	}
	if data == nil {
		return "No text content found", nil
	}
	return restkit.CompactJSON(data), nil
}

// Execute implements toolkit.Tool.
func (a *AgentQLTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return a.Toolkit.Execute(methodName, input)
}

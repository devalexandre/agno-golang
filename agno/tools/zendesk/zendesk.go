// Package zendesk ports Agno's ZendeskTools: searches Zendesk Help Center
// articles (https://{company}.zendesk.com/api/v2/help_center/articles/search.json)
// with HTTP basic auth and returns the article bodies stripped of HTML tags.
package zendesk

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ZendeskTools searches Zendesk Help Center articles.
type ZendeskTools struct {
	toolkit.Toolkit
	username    string
	password    string
	companyName string
	baseURL     string
	enable      bool
	all         bool
	httpClient  *http.Client
}

// Option configures ZendeskTools.
type Option func(*ZendeskTools)

// WithUsername sets the API username (default: ZENDESK_USERNAME). For API
// tokens use "email/token".
func WithUsername(u string) Option { return func(z *ZendeskTools) { z.username = u } }

// WithPassword sets the password or API token (default: ZENDESK_PASSWORD).
func WithPassword(p string) Option { return func(z *ZendeskTools) { z.password = p } }

// WithCompanyName sets the Zendesk subdomain (default: ZENDESK_COMPANY_NAME).
func WithCompanyName(c string) Option { return func(z *ZendeskTools) { z.companyName = c } }

// WithBaseURL overrides https://{company}.zendesk.com.
func WithBaseURL(u string) Option { return func(z *ZendeskTools) { z.baseURL = u } }

// WithEnableSearchZendesk toggles SearchZendesk (default true).
func WithEnableSearchZendesk(b bool) Option { return func(z *ZendeskTools) { z.enable = b } }

// WithAll enables all methods.
func WithAll() Option { return func(z *ZendeskTools) { z.all = true } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(z *ZendeskTools) { z.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(z *ZendeskTools) { z.httpClient = c } }

// SearchParams are the parameters of SearchZendesk.
type SearchParams struct {
	SearchString string `json:"search_string" description:"The search query to look for in Zendesk articles." required:"true"`
}

// New creates the Zendesk toolkit.
func New(opts ...Option) *ZendeskTools {
	z := &ZendeskTools{enable: true}
	z.Toolkit = toolkit.NewToolkit()
	z.Toolkit.Name = "zendesk_tools"
	z.Toolkit.Description = "Search Zendesk Help Center articles."
	for _, o := range opts {
		o(z)
	}
	z.username = restkit.Env(z.username, "ZENDESK_USERNAME")
	z.password = restkit.Env(z.password, "ZENDESK_PASSWORD")
	z.companyName = restkit.Env(z.companyName, "ZENDESK_COMPANY_NAME")
	if z.httpClient == nil {
		z.httpClient = z.Toolkit.HTTPClient()
	}
	if z.all || z.enable {
		z.Toolkit.Register("SearchZendesk", "Searches for articles in Zendesk Help Center that match the given search string and returns them as JSON without HTML tags.", z, z.SearchZendesk, SearchParams{})
	}
	return z
}

var htmlTag = regexp.MustCompile(`<.*?>`)

// SearchZendesk returns a JSON array of article bodies without HTML tags.
func (z *ZendeskTools) SearchZendesk(p SearchParams) (interface{}, error) {
	base := z.baseURL
	if z.username == "" || z.password == "" || (z.companyName == "" && base == "") {
		return "Username, password, or company name not provided.", nil
	}
	if base == "" {
		base = "https://" + z.companyName + ".zendesk.com"
	}
	client := &restkit.Client{HTTP: z.httpClient, BaseURL: base, Name: "Zendesk API", BasicUser: z.username, BasicPass: z.password}
	var resp struct {
		Results []struct {
			Body string `json:"body"`
		} `json:"results"`
	}
	if err := client.Into(http.MethodGet, "/api/v2/help_center/articles/search.json", url.Values{"query": {p.SearchString}}, nil, &resp); err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	articles := []string{}
	for _, a := range resp.Results {
		articles = append(articles, htmlTag.ReplaceAllString(a.Body, ""))
	}
	return restkit.CompactJSON(articles), nil
}

// Execute implements toolkit.Tool.
func (z *ZendeskTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return z.Toolkit.Execute(methodName, input)
}

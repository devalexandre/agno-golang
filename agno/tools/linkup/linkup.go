// Package linkup ports Agno's LinkupTools: real-time web search through the
// Linkup API (POST https://api.linkup.so/v1/search), the REST endpoint wrapped
// by the linkup-sdk Python client.
package linkup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Linkup API base URL.
const DefaultBaseURL = "https://api.linkup.so/v1"

// LinkupTools searches the web with Linkup.
type LinkupTools struct {
	toolkit.Toolkit
	apiKey     string
	baseURL    string
	depth      string
	outputType string
	enableWeb  bool
	all        bool
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures LinkupTools.
type Option func(*LinkupTools)

// WithAPIKey sets the API key (default: LINKUP_API_KEY).
func WithAPIKey(key string) Option { return func(l *LinkupTools) { l.apiKey = key } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(l *LinkupTools) { l.baseURL = u } }

// WithDepth sets the default search depth: "standard" (default) or "deep".
func WithDepth(depth string) Option { return func(l *LinkupTools) { l.depth = depth } }

// WithOutputType sets the default output type: "searchResults" (default) or "sourcedAnswer".
func WithOutputType(t string) Option { return func(l *LinkupTools) { l.outputType = t } }

// WithEnableWebSearchWithLinkup toggles WebSearchWithLinkup (default true).
func WithEnableWebSearchWithLinkup(b bool) Option { return func(l *LinkupTools) { l.enableWeb = b } }

// WithAll enables every tool.
func WithAll() Option { return func(l *LinkupTools) { l.all = true } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(l *LinkupTools) { l.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(l *LinkupTools) { l.httpClient = c } }

// WebSearchParams are the parameters of WebSearchWithLinkup.
type WebSearchParams struct {
	Query      string `json:"query" description:"Query to search for." required:"true"`
	Depth      string `json:"depth,omitempty" description:"(deep|standard) Depth of the search. Defaults to 'standard'."`
	OutputType string `json:"output_type,omitempty" description:"(sourcedAnswer|searchResults) Type of output. Defaults to 'searchResults'."`
}

// New creates the Linkup toolkit.
func New(opts ...Option) *LinkupTools {
	l := &LinkupTools{baseURL: DefaultBaseURL, depth: "standard", outputType: "searchResults", enableWeb: true}
	l.Toolkit = toolkit.NewToolkit()
	l.Toolkit.Name = "linkup_tools"
	l.Toolkit.Description = "Search the web in real time using the Linkup API."
	for _, o := range opts {
		o(l)
	}
	l.apiKey = restkit.Env(l.apiKey, "LINKUP_API_KEY")
	if l.httpClient == nil {
		l.httpClient = l.Toolkit.HTTPClient()
	}
	l.client = &restkit.Client{
		HTTP:    l.httpClient,
		BaseURL: l.baseURL,
		Name:    "Linkup API",
		Headers: map[string]string{"Authorization": "Bearer " + l.apiKey},
	}
	if l.all || l.enableWeb {
		l.Toolkit.Register("WebSearchWithLinkup", "Use this function to search the web for a given query. This function uses the Linkup API to provide realtime online information about the query.", l, l.WebSearchWithLinkup, WebSearchParams{})
	}
	return l
}

// WebSearchWithLinkup runs a Linkup search and returns the API response as
// JSON (search results or a sourced answer), or "Error: ..." on failure.
func (l *LinkupTools) WebSearchWithLinkup(p WebSearchParams) (interface{}, error) {
	if l.apiKey == "" {
		return nil, fmt.Errorf("LINKUP_API_KEY is required")
	}
	depth := p.Depth
	if depth == "" {
		depth = l.depth
	}
	outputType := p.OutputType
	if outputType == "" {
		outputType = l.outputType
	}
	body := map[string]interface{}{"q": p.Query, "depth": depth, "outputType": outputType}
	resp, err := l.client.JSON(http.MethodPost, "/search", nil, body)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return restkit.JSONString(resp), nil
}

// Execute implements toolkit.Tool.
func (l *LinkupTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return l.Toolkit.Execute(methodName, input)
}

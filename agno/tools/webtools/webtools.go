// Package webtools ports Agno's WebTools: expand shortened URLs to their final
// destination by following redirects with HTTP HEAD requests.
package webtools

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultTimeout is the per-attempt timeout used by ExpandUrl (Python: 5s).
const DefaultTimeout = 5 * time.Second

// WebTools holds web utility tools.
type WebTools struct {
	toolkit.Toolkit
	retries         int
	enableExpandURL bool
	all             bool
	httpClient      *http.Client
}

// Option configures WebTools.
type Option func(*WebTools)

// WithRetries sets the number of attempts (default 3).
func WithRetries(n int) Option { return func(w *WebTools) { w.retries = n } }

// WithEnableExpandURL toggles ExpandUrl (default true).
func WithEnableExpandURL(b bool) Option { return func(w *WebTools) { w.enableExpandURL = b } }

// WithAll enables every tool.
func WithAll() Option { return func(w *WebTools) { w.all = true } }

// WithTimeout sets the per-attempt HTTP timeout (default 5s).
func WithTimeout(d time.Duration) Option { return func(w *WebTools) { w.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(w *WebTools) { w.httpClient = c } }

// ExpandURLParams are the parameters of ExpandUrl.
type ExpandURLParams struct {
	URL string `json:"url" description:"The URL to expand." required:"true"`
}

// New creates the web toolkit.
func New(opts ...Option) *WebTools {
	w := &WebTools{retries: 3, enableExpandURL: true}
	w.Toolkit = toolkit.NewToolkit()
	w.Toolkit.Name = "web_tools"
	w.Toolkit.Description = "Web utilities such as expanding shortened URLs."
	w.Toolkit.Timeout = DefaultTimeout
	for _, o := range opts {
		o(w)
	}
	if w.httpClient == nil {
		w.httpClient = w.Toolkit.HTTPClient()
	}
	if w.all || w.enableExpandURL {
		w.Toolkit.Register("ExpandUrl", "Expand a shortened URL to its final destination by following redirects. Returns the final URL, or the original URL on failure.", w, w.ExpandUrl, ExpandURLParams{})
	}
	return w
}

// ExpandUrl follows redirects with HEAD requests and returns the final URL,
// retrying on transport errors; on failure it returns the original URL.
func (w *WebTools) ExpandUrl(params ExpandURLParams) (interface{}, error) {
	retries := w.retries
	if retries < 1 {
		retries = 1
	}
	for attempt := 1; attempt <= retries; attempt++ {
		req, err := http.NewRequest(http.MethodHead, params.URL, nil)
		if err != nil {
			break
		}
		resp, err := w.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		return resp.Request.URL.String(), nil
	}
	return params.URL, nil
}

// Execute implements toolkit.Tool.
func (w *WebTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return w.Toolkit.Execute(methodName, input)
}

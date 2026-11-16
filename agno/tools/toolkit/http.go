package toolkit

import (
	"net/http"
	"time"
)

// DefaultHTTPTimeout is the timeout applied to outbound HTTP calls made by
// toolkits when no explicit timeout is configured. A bounded timeout protects
// agents from hanging forever on slow or malicious endpoints.
const DefaultHTTPTimeout = 30 * time.Second

// NewHTTPClient returns an *http.Client with the given timeout. A zero or
// negative timeout falls back to DefaultHTTPTimeout, so the returned client
// never blocks indefinitely.
func NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = DefaultHTTPTimeout
	}
	return &http.Client{Timeout: timeout}
}

// HTTPClient returns an *http.Client honouring the toolkit's Timeout field
// (DefaultHTTPTimeout when unset). Mirrors Python's Toolkit(timeout=...).
func (tk *Toolkit) HTTPClient() *http.Client {
	return NewHTTPClient(tk.Timeout)
}

// SetTimeout sets the toolkit's I/O timeout used by HTTPClient.
func (tk *Toolkit) SetTimeout(timeout time.Duration) {
	tk.Timeout = timeout
}

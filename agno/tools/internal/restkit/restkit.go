// Package restkit holds small REST helpers shared by the HTTP-only toolkits
// under agno/tools (search, scraping and SaaS integrations ported from Agno).
package restkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// MaxBodyBytes caps the size of response bodies read by the helpers.
const MaxBodyBytes = 20 << 20

// Client is a minimal JSON REST client.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	// Headers are added to every request (e.g. Authorization).
	Headers map[string]string
	// BasicUser/BasicPass, when BasicUser is set, enable HTTP basic auth.
	BasicUser string
	BasicPass string
	// Name prefixes error messages, e.g. "Brave API".
	Name string
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Name   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	name := e.Name
	if name == "" {
		name = "API"
	}
	body := e.Body
	if len(body) > 2000 {
		body = body[:2000] + "..."
	}
	return fmt.Sprintf("%s error (status %d): %s", name, e.Status, strings.TrimSpace(body))
}

// URL joins the base URL, path and query.
func (c *Client) URL(path string, query url.Values) string {
	u := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		u = strings.TrimRight(c.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
		if path == "" {
			u = strings.TrimRight(c.BaseURL, "/")
		}
	}
	if len(query) > 0 {
		if strings.Contains(u, "?") {
			u += "&" + query.Encode()
		} else {
			u += "?" + query.Encode()
		}
	}
	return u
}

// Raw performs a request. body may be nil, []byte, string, url.Values (form
// encoded) or any JSON-marshalable value. It returns the status and raw body;
// non-2xx statuses yield an *APIError.
func (c *Client) Raw(method, path string, query url.Values, body interface{}) (int, []byte, error) {
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case []byte:
		reader = bytes.NewReader(b)
		contentType = "application/json"
	case string:
		reader = strings.NewReader(b)
		contentType = "application/json"
	case url.Values:
		reader = strings.NewReader(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		data, err := json.Marshal(b)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
		contentType = "application/json"
	}
	req, err := http.NewRequest(method, c.URL(path, query), reader)
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if c.BasicUser != "" {
		req.SetBasicAuth(c.BasicUser, c.BasicPass)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%s request failed: %w", c.name(), err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, data, &APIError{Name: c.name(), Status: resp.StatusCode, Body: string(data)}
	}
	return resp.StatusCode, data, nil
}

// JSON performs a request and decodes the JSON response into a generic value.
// An empty body decodes to nil; non-JSON bodies are returned as a string.
func (c *Client) JSON(method, path string, query url.Values, body interface{}) (interface{}, error) {
	_, data, err := c.Raw(method, path, query, body)
	if err != nil {
		return nil, err
	}
	return Decode(data), nil
}

// Into performs a request and decodes the JSON response into out.
func (c *Client) Into(method, path string, query url.Values, body interface{}, out interface{}) error {
	_, data, err := c.Raw(method, path, query, body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to decode %s response: %w", c.name(), err)
	}
	return nil
}

func (c *Client) name() string {
	if c.Name == "" {
		return "API"
	}
	return c.Name
}

// Decode parses JSON bytes into a generic value, falling back to the string.
func Decode(data []byte) interface{} {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	return v
}

// JSONString renders v as indented JSON (Python's json.dumps(..., indent=2)).
func JSONString(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// CompactJSON renders v as compact JSON (Python's json.dumps(...)).
func CompactJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// ErrorJSON renders {"error": msg} like the Python toolkits.
func ErrorJSON(msg string) string {
	return CompactJSON(map[string]string{"error": msg})
}

// Env returns val, or the first non-empty environment variable among names.
func Env(val string, names ...string) string {
	if val != "" {
		return val
	}
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

// Str extracts a string field from a generic JSON object.
func Str(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// Map returns m[key] as an object (nil if absent).
func Map(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, _ := m[key].(map[string]interface{})
	return v
}

// Slice returns m[key] as an array of objects, skipping non-objects.
func Slice(m map[string]interface{}, key string) []map[string]interface{} {
	if m == nil {
		return nil
	}
	arr, _ := m[key].([]interface{})
	out := make([]map[string]interface{}, 0, len(arr))
	for _, it := range arr {
		if obj, ok := it.(map[string]interface{}); ok {
			out = append(out, obj)
		}
	}
	return out
}

// Truncate cuts s to at most n runes (n<=0 = unlimited).
func Truncate(s string, n int) string {
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return string(r[:n])
}

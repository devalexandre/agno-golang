// Package restapi contains small helpers shared by the REST-based toolkits
// under agno/tools. It is internal so toolkits keep a stable public surface.
package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// MaxResponseBytes caps how much of a response body is read.
const MaxResponseBytes = 50 << 20

// Client is a minimal JSON REST client.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	// Headers are added to every request (e.g. Authorization).
	Headers map[string]string
}

// New returns a Client for baseURL using httpClient (http.DefaultClient when nil).
func New(baseURL string, httpClient *http.Client, headers map[string]string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if headers == nil {
		headers = map[string]string{}
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: httpClient, Headers: headers}
}

// APIError is returned for non-2xx responses.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.Status, e.Body)
}

// URL joins the base URL with path (path may already be absolute) and query.
func (c *Client) URL(path string, query url.Values) string {
	u := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		u = c.BaseURL + "/" + strings.TrimLeft(path, "/")
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

// Do sends a request with an optional JSON body and returns the raw response body.
// A non-2xx status yields *APIError.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, error) {
	var reader io.Reader
	contentType := ""
	if body != nil {
		switch b := body.(type) {
		case []byte:
			reader = bytes.NewReader(b)
		case string:
			reader = strings.NewReader(b)
		default:
			data, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request: %w", err)
			}
			reader = bytes.NewReader(data)
			contentType = "application/json"
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL(path, query), reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.Send(req)
}

// Send executes req after applying the client headers.
func (c *Client) Send(req *http.Request) ([]byte, error) {
	for k, v := range c.Headers {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, &APIError{Status: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	return data, nil
}

// JSON performs Do and decodes the response into out (when non-nil).
func (c *Client) JSON(ctx context.Context, method, path string, query url.Values, body, out interface{}) error {
	data, err := c.Do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// FilePart is a file to upload in a multipart form.
type FilePart struct {
	Field    string
	Filename string
	Data     []byte
}

// Multipart sends a multipart/form-data request with fields and files.
func (c *Client) Multipart(ctx context.Context, method, path string, fields map[string]string, files []FilePart) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, err
		}
	}
	for _, f := range files {
		fw, err := w.CreateFormFile(f.Field, f.Filename)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL(path, nil), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return c.Send(req)
}

// ToJSON renders v as indented JSON (falls back to fmt on failure).
func ToJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

// Pretty re-indents a raw JSON payload, returning it unchanged when invalid.
func Pretty(raw []byte) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return ToJSON(v)
}

// ErrorJSON returns {"error": msg} as a string, the convention used by many
// Python toolkits for tool-visible failures.
func ErrorJSON(format string, args ...interface{}) string {
	return ToJSON(map[string]string{"error": fmt.Sprintf(format, args...)})
}

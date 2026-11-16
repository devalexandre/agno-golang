package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Helpers for readers backed by third-party HTTP APIs (Firecrawl, Tavily,
// docling-serve). These talk to a fixed, configured API endpoint, not to
// user-supplied URLs, so they use a plain client.

func apiClient(c *http.Client, timeout time.Duration) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: timeout}
}

// doJSON sends a request with an optional JSON body and decodes a JSON reply into out.
func doJSON(ctx context.Context, client *http.Client, method, url string, headers map[string]string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return sendJSON(client, req, out)
}

func sendJSON(client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("%s %s returned HTTP %d: %s", req.Method, req.URL.Redacted(), resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s response: %w", req.URL.Path, err)
	}
	return nil
}

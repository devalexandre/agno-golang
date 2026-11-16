package reader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// TavilyReader extracts page content through Tavily's Extract API
// (POST https://api.tavily.com/extract). Port of agno-python
// knowledge/reader/tavily_reader.py.
type TavilyReader struct {
	BaseReader
	// APIKey (default: TAVILY_API_KEY env var).
	APIKey string
	// Params are merged into the request body.
	Params map[string]any
	// ExtractFormat is "markdown" (default) or "text".
	ExtractFormat string
	// ExtractDepth is "basic" (default) or "advanced".
	ExtractDepth string
	// AllowedHosts, when non-nil, refuses URLs whose host is not listed.
	AllowedHosts []string
	// BaseURL of the API (default https://api.tavily.com).
	BaseURL    string
	HTTPClient *http.Client
}

// NewTavilyReader creates a Tavily reader.
func NewTavilyReader(apiKey string) *TavilyReader {
	return &TavilyReader{
		BaseReader:    BaseReader{ID: "tavily", Extensions: []string{"http", "https"}},
		APIKey:        apiKey,
		ExtractFormat: "markdown",
		ExtractDepth:  "basic",
		BaseURL:       "https://api.tavily.com",
	}
}

// Read extracts the content of source.
func (r *TavilyReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	if !IsHostAllowed(source, r.AllowedHosts) {
		return nil, fmt.Errorf("tavily: %w: %s", ErrHostNotAllowed, source)
	}
	key := r.APIKey
	if key == "" {
		key = os.Getenv("TAVILY_API_KEY")
	}
	if key == "" {
		return nil, errors.New("tavily: API key not set (TAVILY_API_KEY)")
	}
	format, depth := r.ExtractFormat, r.ExtractDepth
	if format == "" {
		format = "markdown"
	}
	if depth == "" {
		depth = "basic"
	}
	body := map[string]any{"urls": []string{source}, "extract_depth": depth, "format": format}
	for k, v := range r.Params {
		body[k] = v
	}
	base := strings.TrimRight(r.BaseURL, "/")
	if base == "" {
		base = "https://api.tavily.com"
	}
	var resp struct {
		Results []struct {
			URL        string `json:"url"`
			RawContent string `json:"raw_content"`
		} `json:"results"`
		FailedResults []struct {
			URL   string `json:"url"`
			Error string `json:"error"`
		} `json:"failed_results"`
	}
	client := apiClient(r.HTTPClient, 60*time.Second)
	if err := doJSON(ctx, client, http.MethodPost, base+"/extract", map[string]string{"Authorization": "Bearer " + key}, body, &resp); err != nil {
		return nil, fmt.Errorf("tavily: %w", err)
	}
	if len(resp.Results) == 0 {
		reason := "no results"
		if len(resp.FailedResults) > 0 {
			reason = resp.FailedResults[0].Error
		}
		return nil, fmt.Errorf("tavily: extraction failed for %s: %s", source, reason)
	}
	d := document.NewDocument(resp.Results[0].RawContent)
	d.ID = source
	d.Name = source
	d.Source = source
	if format == "markdown" {
		d.ContentType = "text/markdown"
	} else {
		d.ContentType = "text/plain"
	}
	d.Metadata["url"] = source
	return []*document.Document{d}, nil
}

var _ Reader = (*TavilyReader)(nil)

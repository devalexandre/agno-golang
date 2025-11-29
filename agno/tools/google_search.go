package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// GoogleSearchTool provides access to Google Custom Search API
type GoogleSearchTool struct {
	toolkit.Toolkit
	APIKey string
	CX     string // Custom Search Engine ID
}

// NewGoogleSearchTool creates a new Google Search tool
func NewGoogleSearchTool(apiKey, cx string) *GoogleSearchTool {
	t := &GoogleSearchTool{
		APIKey: apiKey,
		CX:     cx,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "GoogleSearch"
	tk.Description = "Search the web using Google Custom Search"

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search Google for a query", t, t.Search, GoogleSearchParams{})

	return t
}

type GoogleSearchParams struct {
	Query string `json:"query" jsonschema:"description=The search query,required=true"`
	Count int    `json:"count" jsonschema:"description=Number of results to return (max 10),default=5"`
}

type GoogleSearchResponse struct {
	Items []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"items"`
}

// Search searches Google for the given query
func (t *GoogleSearchTool) Search(params GoogleSearchParams) (string, error) {
	if t.APIKey == "" || t.CX == "" {
		return "", fmt.Errorf("Google API Key and CX must be set")
	}

	if params.Count <= 0 {
		params.Count = 5
	}
	if params.Count > 10 {
		params.Count = 10
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%d",
		t.APIKey, t.CX, url.QueryEscape(params.Query), params.Count)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to search google: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google search api error: %s", string(body))
	}

	var result GoogleSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Items) == 0 {
		return "No results found", nil
	}

	var sb string
	sb += fmt.Sprintf("Found %d results for '%s':\n\n", len(result.Items), params.Query)

	for i, item := range result.Items {
		sb += fmt.Sprintf("%d. %s\n", i+1, item.Title)
		sb += fmt.Sprintf("   Link: %s\n", item.Link)
		sb += fmt.Sprintf("   Snippet: %s\n\n", item.Snippet)
	}

	return sb, nil
}

// Execute implements the Tool interface
func (t *GoogleSearchTool) Execute(methodName string, args json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, args)
}

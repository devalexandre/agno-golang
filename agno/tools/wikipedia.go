package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// WikipediaTool provides access to Wikipedia content
type WikipediaTool struct {
	toolkit.Toolkit
	Language string
}

// NewWikipediaTool creates a new Wikipedia tool
func NewWikipediaTool() *WikipediaTool {
	t := &WikipediaTool{
		Language: "en",
	}

	tk := toolkit.NewToolkit()
	tk.Name = "Wikipedia"
	tk.Description = "Search and retrieve information from Wikipedia"

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search Wikipedia for a query", t, t.Search, WikipediaSearchParams{})

	return t
}

type WikipediaSearchParams struct {
	Query string `json:"query" jsonschema:"description=The search query,required=true"`
}

// Search searches Wikipedia for the given query
func (t *WikipediaTool) Search(params WikipediaSearchParams) (string, error) {
	// 1. Search for the page title
	searchURL := fmt.Sprintf("https://%s.wikipedia.org/w/api.php?action=opensearch&search=%s&limit=1&namespace=0&format=json",
		t.Language, url.QueryEscape(params.Query))

	resp, err := http.Get(searchURL)
	if err != nil {
		return "", fmt.Errorf("failed to search wikipedia: %w", err)
	}
	defer resp.Body.Close()

	var searchResults []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
		return "", fmt.Errorf("failed to decode search results: %w", err)
	}

	if len(searchResults) < 2 {
		return "No results found", nil
	}

	titles := searchResults[1].([]interface{})
	if len(titles) == 0 {
		return "No results found", nil
	}

	title := titles[0].(string)

	// 2. Get the page content (summary)
	contentURL := fmt.Sprintf("https://%s.wikipedia.org/w/api.php?action=query&prop=extracts&exintro=true&explaintext=true&titles=%s&format=json",
		t.Language, url.QueryEscape(title))

	resp, err = http.Get(contentURL)
	if err != nil {
		return "", fmt.Errorf("failed to get page content: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode page content: %w", err)
	}

	query, ok := result["query"].(map[string]interface{})
	if !ok {
		return "Failed to parse response", nil
	}

	pages, ok := query["pages"].(map[string]interface{})
	if !ok {
		return "Failed to parse pages", nil
	}

	for _, page := range pages {
		p := page.(map[string]interface{})
		if extract, ok := p["extract"].(string); ok {
			return fmt.Sprintf("Wikipedia Summary for '%s':\n%s", title, extract), nil
		}
	}

	return "No content found", nil
}

// Execute implements the Tool interface
func (t *WikipediaTool) Execute(methodName string, args json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, args)
}

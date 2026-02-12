package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const tavilyBaseURL = "https://api.tavily.com"

// TavilyTool provides AI-optimized web search and content extraction via the Tavily API.
type TavilyTool struct {
	toolkit.Toolkit
	apiKey     string
	maxResults int
	httpClient *http.Client
}

// TavilySearchParams defines the parameters for the Search method.
type TavilySearchParams struct {
	Query        string `json:"query" description:"The search query to execute." required:"true"`
	MaxResults   int    `json:"max_results,omitempty" description:"Maximum number of results (1-20). Defaults to toolkit setting."`
	SearchDepth  string `json:"search_depth,omitempty" description:"Search depth: basic, advanced. Default: basic."`
	IncludeAnswer bool  `json:"include_answer,omitempty" description:"Whether to include an AI-generated answer summary."`
	Topic        string `json:"topic,omitempty" description:"Topic filter: general, news, finance. Default: general."`
}

// TavilyExtractParams defines the parameters for the Extract method.
type TavilyExtractParams struct {
	URLs []string `json:"urls" description:"List of URLs to extract content from (max 20)." required:"true"`
}

// NewTavilyTool creates a new Tavily search tool.
// If apiKey is empty, it reads from the TAVILY_API_KEY environment variable.
func NewTavilyTool(apiKey string, maxResults int) *TavilyTool {
	if apiKey == "" {
		apiKey = os.Getenv("TAVILY_API_KEY")
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	t := &TavilyTool{
		apiKey:     apiKey,
		maxResults: maxResults,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "TavilyTool"
	tk.Description = "AI-optimized web search and content extraction using Tavily API."

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search the web using Tavily AI search.", t, t.Search, TavilySearchParams{})
	t.Toolkit.Register("Extract", "Extract content from a list of URLs.", t, t.Extract, TavilyExtractParams{})

	return t
}

// Search performs a web search using the Tavily API.
func (t *TavilyTool) Search(params TavilySearchParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY not set")
	}

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = t.maxResults
	}
	searchDepth := params.SearchDepth
	if searchDepth == "" {
		searchDepth = "basic"
	}
	topic := params.Topic
	if topic == "" {
		topic = "general"
	}

	reqBody := map[string]interface{}{
		"query":          params.Query,
		"max_results":    maxResults,
		"search_depth":   searchDepth,
		"include_answer": params.IncludeAnswer,
		"topic":          topic,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", tavilyBaseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

// Extract extracts content from a list of URLs using the Tavily API.
func (t *TavilyTool) Extract(params TavilyExtractParams) (interface{}, error) {
	if len(params.URLs) == 0 {
		return nil, fmt.Errorf("at least one URL is required")
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("TAVILY_API_KEY not set")
	}

	reqBody := map[string]interface{}{
		"urls": params.URLs,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", tavilyBaseURL+"/extract", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily extract request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

// Execute implements the toolkit.Tool interface.
func (t *TavilyTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

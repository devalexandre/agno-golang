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

const serperBaseURL = "https://google.serper.dev"

// SerperTool provides Google search results via the Serper API.
type SerperTool struct {
	toolkit.Toolkit
	apiKey     string
	httpClient *http.Client
}

// SerperSearchParams defines the parameters for web search.
type SerperSearchParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of results (default: 10, max: 100)."`
	Country    string `json:"gl,omitempty" description:"Country code (e.g., 'us'). Default: us."`
	Language   string `json:"hl,omitempty" description:"Language code (e.g., 'en'). Default: en."`
}

// SerperNewsParams defines the parameters for news search.
type SerperNewsParams struct {
	Query      string `json:"query" description:"The news search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of news results. Default: 10."`
	TimeRange  string `json:"tbs,omitempty" description:"Time range: qdr:h (hour), qdr:d (day), qdr:w (week), qdr:m (month), qdr:y (year)."`
}

// SerperImagesParams defines the parameters for image search.
type SerperImagesParams struct {
	Query      string `json:"query" description:"The image search query." required:"true"`
	NumResults int    `json:"num_results,omitempty" description:"Number of image results. Default: 10."`
}

// NewSerperTool creates a new Serper search tool.
// If apiKey is empty, it reads from the SERPER_API_KEY environment variable.
func NewSerperTool(apiKey string) *SerperTool {
	if apiKey == "" {
		apiKey = os.Getenv("SERPER_API_KEY")
	}

	t := &SerperTool{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "SerperTool"
	tk.Description = "Fast Google search results via Serper API: web search, news, and images."

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search Google for web results.", t, t.Search, SerperSearchParams{})
	t.Toolkit.Register("News", "Search Google News for recent articles.", t, t.News, SerperNewsParams{})
	t.Toolkit.Register("Images", "Search Google Images.", t, t.Images, SerperImagesParams{})

	return t
}

func (t *SerperTool) doRequest(endpoint string, reqBody map[string]interface{}) (interface{}, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("SERPER_API_KEY not set")
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", serperBaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-API-KEY", t.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("serper request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serper API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

// Search performs a Google web search.
func (t *SerperTool) Search(params SerperSearchParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	reqBody := map[string]interface{}{
		"q": params.Query,
	}
	if params.NumResults > 0 {
		reqBody["num"] = params.NumResults
	}
	if params.Country != "" {
		reqBody["gl"] = params.Country
	}
	if params.Language != "" {
		reqBody["hl"] = params.Language
	}

	return t.doRequest("/search", reqBody)
}

// News performs a Google News search.
func (t *SerperTool) News(params SerperNewsParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	reqBody := map[string]interface{}{
		"q": params.Query,
	}
	if params.NumResults > 0 {
		reqBody["num"] = params.NumResults
	}
	if params.TimeRange != "" {
		reqBody["tbs"] = params.TimeRange
	}

	return t.doRequest("/news", reqBody)
}

// Images performs a Google Images search.
func (t *SerperTool) Images(params SerperImagesParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	reqBody := map[string]interface{}{
		"q": params.Query,
	}
	if params.NumResults > 0 {
		reqBody["num"] = params.NumResults
	}

	return t.doRequest("/images", reqBody)
}

// Execute implements the toolkit.Tool interface.
func (t *SerperTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

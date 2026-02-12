package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const serpapiBaseURL = "https://serpapi.com/search.json"

// SerpAPITool provides search engine results via the SerpAPI service.
type SerpAPITool struct {
	toolkit.Toolkit
	apiKey string
	httpClient *http.Client
}

// SerpAPISearchParams defines the parameters for the Search method.
type SerpAPISearchParams struct {
	Query      string `json:"query" description:"The search query." required:"true"`
	Engine     string `json:"engine,omitempty" description:"Search engine to use: google, bing, yahoo, duckduckgo, baidu, yandex. Default: google."`
	NumResults int    `json:"num_results,omitempty" description:"Number of results to return. Default: 10."`
	Location   string `json:"location,omitempty" description:"Geographic location for results (e.g., 'New York')."`
	Language   string `json:"hl,omitempty" description:"Interface language code (e.g., 'en'). Default: en."`
	Country    string `json:"gl,omitempty" description:"Country code (e.g., 'us'). Default: us."`
}

// NewSerpAPITool creates a new SerpAPI search tool.
// If apiKey is empty, it reads from the SERPAPI_API_KEY environment variable.
func NewSerpAPITool(apiKey string) *SerpAPITool {
	if apiKey == "" {
		apiKey = os.Getenv("SERPAPI_API_KEY")
	}

	t := &SerpAPITool{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "SerpAPITool"
	tk.Description = "Search engine results from Google, Bing and others via SerpAPI."

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search using SerpAPI for web results.", t, t.Search, SerpAPISearchParams{})

	return t
}

// Search performs a search using SerpAPI.
func (t *SerpAPITool) Search(params SerpAPISearchParams) (interface{}, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("SERPAPI_API_KEY not set")
	}

	engine := params.Engine
	if engine == "" {
		engine = "google"
	}
	num := params.NumResults
	if num <= 0 {
		num = 10
	}
	hl := params.Language
	if hl == "" {
		hl = "en"
	}
	gl := params.Country
	if gl == "" {
		gl = "us"
	}

	u, err := url.Parse(serpapiBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	q := u.Query()
	q.Set("q", params.Query)
	q.Set("api_key", t.apiKey)
	q.Set("engine", engine)
	q.Set("num", strconv.Itoa(num))
	q.Set("hl", hl)
	q.Set("gl", gl)
	if params.Location != "" {
		q.Set("location", params.Location)
	}
	u.RawQuery = q.Encode()

	resp, err := t.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("serpapi request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serpapi error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	return result, nil
}

// Execute implements the toolkit.Tool interface.
func (t *SerpAPITool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

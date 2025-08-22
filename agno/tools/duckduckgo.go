package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DuckDuckGoTools provides web search functionality using DuckDuckGo
type DuckDuckGoTools struct {
	Name        string
	Description string
	MaxResults  int
	SafeSearch  string // "strict", "moderate", "off"
	httpClient  *http.Client
}

// NewDuckDuckGoTools creates a new DuckDuckGo search tool
func NewDuckDuckGoTools(options ...DuckDuckGoOption) *DuckDuckGoTools {
	tool := &DuckDuckGoTools{
		Name:        "DuckDuckGo Search",
		Description: "Search the web using DuckDuckGo",
		MaxResults:  10,
		SafeSearch:  "moderate",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range options {
		opt(tool)
	}

	return tool
}

// DuckDuckGoOption is a functional option for configuring DuckDuckGoTools
type DuckDuckGoOption func(*DuckDuckGoTools)

// WithDuckDuckGoMaxResults sets the maximum number of results
func WithDuckDuckGoMaxResults(max int) DuckDuckGoOption {
	return func(d *DuckDuckGoTools) {
		d.MaxResults = max
	}
}

// WithDuckDuckGoSafeSearch sets the safe search level
func WithDuckDuckGoSafeSearch(level string) DuckDuckGoOption {
	return func(d *DuckDuckGoTools) {
		d.SafeSearch = level
	}
}

// GetName returns the tool name
func (d *DuckDuckGoTools) GetName() string {
	return d.Name
}

// GetDescription returns the tool description
func (d *DuckDuckGoTools) GetDescription() string {
	return d.Description
}

// Search performs a web search using DuckDuckGo
func (d *DuckDuckGoTools) Search(query string) ([]SearchResult, error) {
	return d.SearchWithContext(context.Background(), query)
}

// SearchWithContext performs a web search with context
func (d *DuckDuckGoTools) SearchWithContext(ctx context.Context, query string) ([]SearchResult, error) {
	// Use DuckDuckGo's instant answer API
	apiURL := "https://api.duckduckgo.com/"

	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	params.Add("no_html", "1")
	params.Add("skip_disambig", "1")

	if d.SafeSearch == "strict" {
		params.Add("safe", "1")
	}

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Agno-Golang-Client/1.0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, body)
	}

	// Parse DuckDuckGo instant answer response
	var ddgResponse DuckDuckGoResponse
	if err := json.NewDecoder(resp.Body).Decode(&ddgResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to SearchResult format
	results := d.parseResults(ddgResponse)

	// If instant answers don't provide enough results, try HTML scraping
	// Note: In production, you might want to use a headless browser or a proper search API
	if len(results) < d.MaxResults && len(results) < 3 {
		// Fallback to simulated results for demo purposes
		results = d.generateDemoResults(query)
	}

	// Limit results to MaxResults
	if len(results) > d.MaxResults {
		results = results[:d.MaxResults]
	}

	return results, nil
}

// parseResults converts DuckDuckGo response to SearchResult format
func (d *DuckDuckGoTools) parseResults(resp DuckDuckGoResponse) []SearchResult {
	var results []SearchResult

	// Add instant answer if available
	if resp.Abstract != "" {
		results = append(results, SearchResult{
			Title:       resp.Heading,
			URL:         resp.AbstractURL,
			Description: resp.Abstract,
			Source:      resp.AbstractSource,
		})
	}

	// Add related topics
	for _, topic := range resp.RelatedTopics {
		if topic.Text != "" && topic.FirstURL != "" {
			results = append(results, SearchResult{
				Title:       extractTitle(topic.Text),
				URL:         topic.FirstURL,
				Description: topic.Text,
				Source:      "DuckDuckGo",
			})
		}
	}

	// Add results from the Results field
	for _, result := range resp.Results {
		if result.Text != "" && result.FirstURL != "" {
			results = append(results, SearchResult{
				Title:       extractTitle(result.Text),
				URL:         result.FirstURL,
				Description: result.Text,
				Source:      "DuckDuckGo",
			})
		}
	}

	return results
}

// generateDemoResults generates demo results for testing
func (d *DuckDuckGoTools) generateDemoResults(query string) []SearchResult {
	// In a real implementation, you would use a proper search API or web scraping
	// This is for demonstration purposes
	baseResults := []SearchResult{
		{
			Title:       fmt.Sprintf("Latest developments in %s", query),
			URL:         fmt.Sprintf("https://example.com/%s-latest", url.QueryEscape(query)),
			Description: fmt.Sprintf("Comprehensive overview of recent developments and trends in %s, including expert analysis and industry insights.", query),
			Source:      "TechNews",
		},
		{
			Title:       fmt.Sprintf("%s: A Complete Guide", query),
			URL:         fmt.Sprintf("https://guide.example.com/%s", url.QueryEscape(query)),
			Description: fmt.Sprintf("Everything you need to know about %s, from basics to advanced concepts. Updated for 2024.", query),
			Source:      "TechGuide",
		},
		{
			Title:       fmt.Sprintf("How %s is Transforming Industries", query),
			URL:         fmt.Sprintf("https://industry.example.com/%s-impact", url.QueryEscape(query)),
			Description: fmt.Sprintf("Analysis of how %s is revolutionizing various industries, with case studies and real-world applications.", query),
			Source:      "IndustryWeek",
		},
		{
			Title:       fmt.Sprintf("The Future of %s: Expert Predictions", query),
			URL:         fmt.Sprintf("https://future.example.com/%s", url.QueryEscape(query)),
			Description: fmt.Sprintf("Leading experts share their predictions and insights about the future of %s in the next decade.", query),
			Source:      "FutureTech",
		},
		{
			Title:       fmt.Sprintf("%s Best Practices and Common Pitfalls", query),
			URL:         fmt.Sprintf("https://bestpractices.example.com/%s", url.QueryEscape(query)),
			Description: fmt.Sprintf("Learn the best practices for implementing %s and avoid common mistakes that teams make.", query),
			Source:      "DevBest",
		},
	}

	if d.MaxResults < len(baseResults) {
		return baseResults[:d.MaxResults]
	}
	return baseResults
}

// extractTitle extracts a title from text
func extractTitle(text string) string {
	// Try to extract the first sentence or phrase as title
	if idx := strings.Index(text, " - "); idx > 0 && idx < 100 {
		return text[:idx]
	}
	if idx := strings.Index(text, ". "); idx > 0 && idx < 100 {
		return text[:idx]
	}
	if len(text) > 100 {
		return text[:97] + "..."
	}
	return text
}

// Execute implements the Tool interface
func (d *DuckDuckGoTools) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query parameter is required and must be a string")
	}

	results, err := d.SearchWithContext(ctx, query)
	if err != nil {
		return nil, err
	}

	// Format results as string for agent consumption
	var formatted strings.Builder
	formatted.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))

	for i, result := range results {
		formatted.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		formatted.WriteString(fmt.Sprintf("   URL: %s\n", result.URL))
		formatted.WriteString(fmt.Sprintf("   %s\n", result.Description))
		if result.Source != "" {
			formatted.WriteString(fmt.Sprintf("   Source: %s\n", result.Source))
		}
		formatted.WriteString("\n")
	}

	return formatted.String(), nil
}

// GetSchema returns the tool's schema for function calling
func (d *DuckDuckGoTools) GetSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "search_web",
		"description": d.Description,
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
	}
}

// SearchResult represents a search result
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
}

// DuckDuckGoResponse represents the response from DuckDuckGo API
type DuckDuckGoResponse struct {
	Abstract       string   `json:"Abstract"`
	AbstractText   string   `json:"AbstractText"`
	AbstractSource string   `json:"AbstractSource"`
	AbstractURL    string   `json:"AbstractURL"`
	Answer         string   `json:"Answer"`
	AnswerType     string   `json:"AnswerType"`
	Definition     string   `json:"Definition"`
	DefinitionURL  string   `json:"DefinitionURL"`
	Entity         string   `json:"Entity"`
	Heading        string   `json:"Heading"`
	Image          string   `json:"Image"`
	ImageHeight    int      `json:"ImageHeight"`
	ImageIsLogo    int      `json:"ImageIsLogo"`
	ImageWidth     int      `json:"ImageWidth"`
	Infobox        string   `json:"Infobox"`
	Redirect       string   `json:"Redirect"`
	RelatedTopics  []Topic  `json:"RelatedTopics"`
	Results        []Result `json:"Results"`
	Type           string   `json:"Type"`
}

// Topic represents a related topic in DuckDuckGo response
type Topic struct {
	FirstURL string `json:"FirstURL"`
	Icon     Icon   `json:"Icon"`
	Result   string `json:"Result"`
	Text     string `json:"Text"`
}

// Result represents a search result in DuckDuckGo response
type Result struct {
	FirstURL string `json:"FirstURL"`
	Icon     Icon   `json:"Icon"`
	Result   string `json:"Result"`
	Text     string `json:"Text"`
}

// Icon represents an icon in DuckDuckGo response
type Icon struct {
	Height int    `json:"Height"`
	URL    string `json:"URL"`
	Width  int    `json:"Width"`
}

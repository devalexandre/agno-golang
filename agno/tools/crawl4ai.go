package tools

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"

	"golang.org/x/net/html"
)

// Crawl4AITool provides web crawling with content extraction.
// Unlike Firecrawl, this tool works without an API key by fetching and parsing HTML directly.
type Crawl4AITool struct {
	toolkit.Toolkit
	httpClient *http.Client
	maxDepth   int
}

// Crawl4AICrawlParams defines the parameters for the Crawl method.
type Crawl4AICrawlParams struct {
	URL      string `json:"url" description:"The URL to crawl and extract content from." required:"true"`
	MaxPages int    `json:"max_pages,omitempty" description:"Maximum number of pages to crawl. Default: 1."`
}

// NewCrawl4AITool creates a new Crawl4AI tool.
func NewCrawl4AITool(maxDepth int) *Crawl4AITool {
	if maxDepth <= 0 {
		maxDepth = 1
	}

	t := &Crawl4AITool{
		httpClient: &http.Client{},
		maxDepth:   maxDepth,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "Crawl4AITool"
	tk.Description = "Web crawling with intelligent content extraction. Fetches pages and extracts clean text."

	t.Toolkit = tk
	t.Toolkit.Register("Crawl", "Crawl a URL and extract its main text content.", t, t.Crawl, Crawl4AICrawlParams{})

	return t
}

// Crawl fetches and extracts text content from a URL.
func (t *Crawl4AITool) Crawl(params Crawl4AICrawlParams) (interface{}, error) {
	if params.URL == "" {
		return nil, fmt.Errorf("url is required")
	}

	maxPages := params.MaxPages
	if maxPages <= 0 {
		maxPages = t.maxDepth
	}

	type pageResult struct {
		URL     string `json:"url"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	visited := make(map[string]bool)
	var results []pageResult
	queue := []string{params.URL}

	for len(queue) > 0 && len(results) < maxPages {
		currentURL := queue[0]
		queue = queue[1:]

		if visited[currentURL] {
			continue
		}
		visited[currentURL] = true

		resp, err := t.httpClient.Get(currentURL)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		doc, err := html.Parse(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		title := htmlExtractTitle(doc)
		content := htmlExtractTextContent(doc)
		links := htmlExtractLinks(doc, currentURL)

		results = append(results, pageResult{
			URL:     currentURL,
			Title:   title,
			Content: content,
		})

		for _, link := range links {
			if !visited[link] && len(results)+len(queue) < maxPages {
				queue = append(queue, link)
			}
		}
	}

	if len(results) == 0 {
		return "No content could be extracted.", nil
	}

	output, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(output), nil
}

// Execute implements the toolkit.Tool interface.
func (t *Crawl4AITool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

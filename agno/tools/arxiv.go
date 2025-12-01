package tools

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// ArxivTool provides access to Arxiv papers
type ArxivTool struct {
	toolkit.Toolkit
	MaxResults int
}

// NewArxivTool creates a new Arxiv tool
func NewArxivTool(maxResults int) *ArxivTool {
	if maxResults <= 0 {
		maxResults = 5
	}

	t := &ArxivTool{
		MaxResults: maxResults,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "Arxiv"
	tk.Description = "Search for academic papers on Arxiv"

	t.Toolkit = tk
	t.Toolkit.Register("Search", "Search Arxiv for papers", t, t.Search, ArxivSearchParams{})

	return t
}

type ArxivSearchParams struct {
	Query string `json:"query" jsonschema:"description=The search query (e.g. 'quantum computing', 'au:del_maestro'),required=true"`
}

// ArxivResponse represents the XML response from Arxiv API
type ArxivResponse struct {
	Entry []struct {
		Title   string `xml:"title"`
		Summary string `xml:"summary"`
		Author  []struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Link []struct {
			Href  string `xml:"href,attr"`
			Rel   string `xml:"rel,attr"`
			Title string `xml:"title,attr"`
		} `xml:"link"`
		Published string `xml:"published"`
	} `xml:"entry"`
}

// Search searches Arxiv for the given query
func (t *ArxivTool) Search(params ArxivSearchParams) (string, error) {
	apiURL := fmt.Sprintf("http://export.arxiv.org/api/query?search_query=all:%s&start=0&max_results=%d",
		url.QueryEscape(params.Query), t.MaxResults)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to search arxiv: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var result ArxivResponse
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode xml response: %w", err)
	}

	if len(result.Entry) == 0 {
		return "No papers found", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d papers for '%s':\n\n", len(result.Entry), params.Query))

	for i, entry := range result.Entry {
		// Format authors
		var authors []string
		for _, a := range entry.Author {
			authors = append(authors, a.Name)
		}

		// Find PDF link
		pdfLink := ""
		for _, l := range entry.Link {
			if l.Rel == "alternate" { // Usually the abstract page
				pdfLink = l.Href
			}
			if l.Title == "pdf" {
				pdfLink = l.Href
			}
		}

		// Clean up title and summary (remove newlines)
		title := strings.ReplaceAll(strings.TrimSpace(entry.Title), "\n", " ")
		summary := strings.ReplaceAll(strings.TrimSpace(entry.Summary), "\n", " ")
		if len(summary) > 300 {
			summary = summary[:297] + "..."
		}

		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, title))
		sb.WriteString(fmt.Sprintf("   Authors: %s\n", strings.Join(authors, ", ")))
		sb.WriteString(fmt.Sprintf("   Published: %s\n", entry.Published[:10]))
		sb.WriteString(fmt.Sprintf("   Link: %s\n", pdfLink))
		sb.WriteString(fmt.Sprintf("   Summary: %s\n\n", summary))
	}

	return sb.String(), nil
}

// Execute implements the Tool interface
func (t *ArxivTool) Execute(methodName string, args json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, args)
}

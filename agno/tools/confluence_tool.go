// Package tools provides various agent tools, including Confluence integration.
package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Input struct for search
type SearchConfluenceInput struct {
	Query string `json:"query" description:"CQL query for Confluence search." required:"true"`
}

// Input struct for get_page_content
type GetPageContentInput struct {
	PageID string `json:"page_id" description:"Confluence page ID." required:"true"`
}

// ConfluenceTool provides methods to interact with Atlassian Confluence REST API.
type ConfluenceTool struct {
	BaseURL  string
	Username string
	APIToken string
	toolkit.Toolkit
}

// NewConfluenceTool creates a new ConfluenceTool instance.
func NewConfluenceTool(baseURL, username, apiToken string) toolkit.Tool {
	tk := toolkit.NewToolkit()
	tk.Name = "ConfluenceTool"
	tk.Description = "Toolkit for Atlassian Confluence: search and retrieve page content."

	c := &ConfluenceTool{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Username: username,
		APIToken: apiToken,
		Toolkit:  tk,
	}

	c.Toolkit.Register("SearchConfluence", c, c.SearchConfluence, SearchConfluenceInput{})
	c.Toolkit.Register("GetPageContent", c, c.GetPageContent, GetPageContentInput{})

	return c
}

// SearchConfluence queries Confluence for pages matching the query string.
func (c *ConfluenceTool) SearchConfluence(input SearchConfluenceInput) (interface{}, error) {
	cql := input.Query
	// If the query does not look like a CQL expression, wrap it as a text search
	if !strings.ContainsAny(cql, "=~<>") {
		cql = fmt.Sprintf("text~\"%s\"", cql)
	}
	fmt.Printf("[ConfluenceTool] SearchConfluence: Final CQL: %s\n", cql)
	url := fmt.Sprintf("%s/wiki/rest/api/content/search?cql=%s", c.BaseURL, urlEncode(cql))
	fmt.Printf("[ConfluenceTool] SearchConfluence: URL: %s\n", url)
	fmt.Printf("[ConfluenceTool] SearchConfluence: Username: %s\n", c.Username)
	fmt.Printf("[ConfluenceTool] SearchConfluence: Query: %s\n", input.Query)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[ConfluenceTool] SearchConfluence: Error creating request: %v\n", err)
		return nil, err
	}
	c.setAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("[ConfluenceTool] SearchConfluence: Error on request: %v\n", err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	fmt.Printf("[ConfluenceTool] SearchConfluence: Response Status: %s\n", resp.Status)
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("[ConfluenceTool] SearchConfluence: Error Body: %s\n", string(body))
		return nil, fmt.Errorf("confluence search failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[ConfluenceTool] SearchConfluence: Error reading body: %v\n", err)
		return nil, err
	}
	fmt.Printf("[ConfluenceTool] SearchConfluence: Response Body: %s\n", string(body))
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("[ConfluenceTool] SearchConfluence: Error unmarshalling body: %v\n", err)
		return nil, err
	}
	return result, nil
}

// GetPageContent retrieves the content of a Confluence page by ID.
func (c *ConfluenceTool) GetPageContent(input GetPageContentInput) (interface{}, error) {
	url := fmt.Sprintf("%s/wiki/rest/api/content/%s?expand=body.storage", c.BaseURL, input.PageID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("confluence get_page_content failed: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// setAuth sets basic auth for the request.
func (c *ConfluenceTool) setAuth(req *http.Request) {
	req.SetBasicAuth(c.Username, c.APIToken)
	req.Header.Set("Accept", "application/json")
}

// urlEncode encodes a string for use in a URL query.
func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}

var _ toolkit.Tool = (*ConfluenceTool)(nil)

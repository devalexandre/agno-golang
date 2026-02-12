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

const notionBaseURL = "https://api.notion.com/v1"

// NotionTool provides integration with Notion for managing pages and databases.
type NotionTool struct {
	toolkit.Toolkit
	apiKey     string
	httpClient *http.Client
}

type NotionSearchParams struct {
	Query string `json:"query" description:"Search query to find pages and databases." required:"true"`
}

type NotionGetPageParams struct {
	PageID string `json:"page_id" description:"The Notion page ID." required:"true"`
}

type NotionCreatePageParams struct {
	ParentID string `json:"parent_id" description:"Parent page or database ID." required:"true"`
	Title    string `json:"title" description:"Page title." required:"true"`
	Content  string `json:"content,omitempty" description:"Page content text."`
}

type NotionQueryDatabaseParams struct {
	DatabaseID string `json:"database_id" description:"The Notion database ID." required:"true"`
	Filter     string `json:"filter,omitempty" description:"JSON filter object for the database query."`
}

// NewNotionTool creates a new Notion tool.
// If apiKey is empty, it reads from the NOTION_API_KEY environment variable.
func NewNotionTool(apiKey string) *NotionTool {
	if apiKey == "" {
		apiKey = os.Getenv("NOTION_API_KEY")
	}

	t := &NotionTool{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "NotionTool"
	tk.Description = "Manage Notion pages and databases: search, create, read, and query."

	t.Toolkit = tk
	t.Toolkit.Register("SearchPages", "Search Notion for pages and databases.", t, t.SearchPages, NotionSearchParams{})
	t.Toolkit.Register("GetPage", "Get a Notion page by ID.", t, t.GetPage, NotionGetPageParams{})
	t.Toolkit.Register("CreatePage", "Create a new Notion page.", t, t.CreatePage, NotionCreatePageParams{})
	t.Toolkit.Register("QueryDatabase", "Query a Notion database.", t, t.QueryDatabase, NotionQueryDatabaseParams{})

	return t
}

func (t *NotionTool) doRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("NOTION_API_KEY not set")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, notionBaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("notion request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("notion API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (t *NotionTool) SearchPages(params NotionSearchParams) (interface{}, error) {
	body := map[string]interface{}{
		"query": params.Query,
	}
	return t.doRequest("POST", "/search", body)
}

func (t *NotionTool) GetPage(params NotionGetPageParams) (interface{}, error) {
	return t.doRequest("GET", "/pages/"+params.PageID, nil)
}

func (t *NotionTool) CreatePage(params NotionCreatePageParams) (interface{}, error) {
	body := map[string]interface{}{
		"parent": map[string]string{
			"page_id": params.ParentID,
		},
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]string{"content": params.Title}},
				},
			},
		},
	}

	if params.Content != "" {
		body["children"] = []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{"text": map[string]string{"content": params.Content}},
					},
				},
			},
		}
	}

	return t.doRequest("POST", "/pages", body)
}

func (t *NotionTool) QueryDatabase(params NotionQueryDatabaseParams) (interface{}, error) {
	body := map[string]interface{}{}

	if params.Filter != "" {
		var filter interface{}
		if err := json.Unmarshal([]byte(params.Filter), &filter); err == nil {
			body["filter"] = filter
		}
	}

	return t.doRequest("POST", "/databases/"+params.DatabaseID+"/query", body)
}

func (t *NotionTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

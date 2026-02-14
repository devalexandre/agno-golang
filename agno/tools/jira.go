package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// JiraTool provides issue tracking integration with Jira.
type JiraTool struct {
	toolkit.Toolkit
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

type JiraCreateIssueParams struct {
	Project     string `json:"project" description:"The Jira project key (e.g., 'PROJ')." required:"true"`
	Summary     string `json:"summary" description:"Issue summary/title." required:"true"`
	Description string `json:"description,omitempty" description:"Detailed issue description."`
	IssueType   string `json:"issue_type,omitempty" description:"Issue type: Task, Bug, Story, Epic. Default: Task."`
}

type JiraGetIssueParams struct {
	IssueKey string `json:"issue_key" description:"The Jira issue key (e.g., 'PROJ-123')." required:"true"`
}

type JiraSearchParams struct {
	JQL        string `json:"jql" description:"JQL query to search issues (e.g., 'project = PROJ AND status = Open')." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum results. Default: 10."`
}

type JiraAddCommentParams struct {
	IssueKey string `json:"issue_key" description:"The Jira issue key." required:"true"`
	Comment  string `json:"comment" description:"The comment text to add." required:"true"`
}

// NewJiraTool creates a new Jira tool.
// If parameters are empty, they are read from environment variables:
// JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN.
func NewJiraTool(baseURL, email, apiToken string) *JiraTool {
	if baseURL == "" {
		baseURL = os.Getenv("JIRA_URL")
	}
	if email == "" {
		email = os.Getenv("JIRA_EMAIL")
	}
	if apiToken == "" {
		apiToken = os.Getenv("JIRA_API_TOKEN")
	}

	t := &JiraTool{
		baseURL:    baseURL,
		email:      email,
		apiToken:   apiToken,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "JiraTool"
	tk.Description = "Issue tracking with Jira: create, search, read, and comment on issues."

	t.Toolkit = tk
	t.Toolkit.Register("CreateIssue", "Create a new Jira issue.", t, t.CreateIssue, JiraCreateIssueParams{})
	t.Toolkit.Register("GetIssue", "Get details of a Jira issue.", t, t.GetIssue, JiraGetIssueParams{})
	t.Toolkit.Register("SearchIssues", "Search Jira issues using JQL.", t, t.SearchIssues, JiraSearchParams{})
	t.Toolkit.Register("AddComment", "Add a comment to a Jira issue.", t, t.AddComment, JiraAddCommentParams{})

	return t
}

func (t *JiraTool) doRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	if t.baseURL == "" || t.email == "" || t.apiToken == "" {
		return nil, fmt.Errorf("JIRA_URL, JIRA_EMAIL, and JIRA_API_TOKEN must be set")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, t.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(t.email, t.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jira API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func (t *JiraTool) CreateIssue(params JiraCreateIssueParams) (interface{}, error) {
	issueType := params.IssueType
	if issueType == "" {
		issueType = "Task"
	}

	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":   map[string]string{"key": params.Project},
			"summary":   params.Summary,
			"issuetype": map[string]string{"name": issueType},
		},
	}

	if params.Description != "" {
		fields := body["fields"].(map[string]interface{})
		fields["description"] = map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{"type": "text", "text": params.Description},
					},
				},
			},
		}
	}

	return t.doRequest("POST", "/rest/api/3/issue", body)
}

func (t *JiraTool) GetIssue(params JiraGetIssueParams) (interface{}, error) {
	return t.doRequest("GET", "/rest/api/3/issue/"+url.PathEscape(params.IssueKey), nil)
}

func (t *JiraTool) SearchIssues(params JiraSearchParams) (interface{}, error) {
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	body := map[string]interface{}{
		"jql":        params.JQL,
		"maxResults": maxResults,
	}

	return t.doRequest("POST", "/rest/api/3/search", body)
}

func (t *JiraTool) AddComment(params JiraAddCommentParams) (interface{}, error) {
	body := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{"type": "text", "text": params.Comment},
					},
				},
			},
		},
	}

	path := fmt.Sprintf("/rest/api/3/issue/%s/comment", url.PathEscape(params.IssueKey))
	return t.doRequest("POST", path, body)
}

func (t *JiraTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

// Package linear ports Agno's LinearTools: users, teams and issues of a Linear
// workspace through the Linear GraphQL API (https://api.linear.app/graphql).
package linear

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultEndpoint is the Linear GraphQL endpoint.
const DefaultEndpoint = "https://api.linear.app/graphql"

// LinearTools queries and mutates Linear issues.
type LinearTools struct {
	toolkit.Toolkit
	apiKey     string
	endpoint   string
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures LinearTools.
type Option func(*LinearTools)

// WithAPIKey sets the API key (default: LINEAR_API_KEY).
func WithAPIKey(key string) Option { return func(l *LinearTools) { l.apiKey = key } }

// WithBaseURL overrides the GraphQL endpoint URL.
func WithBaseURL(u string) Option { return func(l *LinearTools) { l.endpoint = u } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(l *LinearTools) { l.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(l *LinearTools) { l.httpClient = c } }

// NoParams is used by methods without arguments.
type NoParams struct{}

// IssueParams identify an issue.
type IssueParams struct {
	IssueID string `json:"issue_id" description:"The unique identifier of the issue to retrieve." required:"true"`
}

// CreateIssueParams are the parameters of CreateIssue.
type CreateIssueParams struct {
	Title       string `json:"title" description:"The title of the new issue." required:"true"`
	Description string `json:"description" description:"The description of the new issue." required:"true"`
	TeamID      string `json:"team_id" description:"The unique identifier of the team in which to create the issue." required:"true"`
	ProjectID   string `json:"project_id,omitempty" description:"The ID of the project (optional)."`
	AssigneeID  string `json:"assignee_id,omitempty" description:"The ID of the assignee (optional)."`
}

// UpdateIssueParams are the parameters of UpdateIssue.
type UpdateIssueParams struct {
	IssueID string `json:"issue_id" description:"The unique identifier of the issue to update." required:"true"`
	Title   string `json:"title" description:"The new title for the issue." required:"true"`
}

// UserParams identify a user.
type UserParams struct {
	UserID string `json:"user_id" description:"The unique identifier of the user for whom to retrieve assigned issues." required:"true"`
}

// WorkflowParams identify a workflow state.
type WorkflowParams struct {
	WorkflowID string `json:"workflow_id" description:"The unique identifier of the workflow state to retrieve issues from." required:"true"`
}

// New creates the Linear toolkit.
func New(opts ...Option) *LinearTools {
	l := &LinearTools{endpoint: DefaultEndpoint}
	l.Toolkit = toolkit.NewToolkit()
	l.Toolkit.Name = "linear_tools"
	l.Toolkit.Description = "Read and manage Linear users, teams and issues."
	for _, o := range opts {
		o(l)
	}
	l.apiKey = restkit.Env(l.apiKey, "LINEAR_API_KEY")
	if l.httpClient == nil {
		l.httpClient = l.Toolkit.HTTPClient()
	}
	l.client = &restkit.Client{HTTP: l.httpClient, BaseURL: l.endpoint, Name: "Linear API", Headers: map[string]string{"Authorization": l.apiKey}}

	l.Toolkit.Register("GetUserDetails", "Fetch the authenticated user's id, name and email.", l, l.GetUserDetails, NoParams{})
	l.Toolkit.Register("GetTeamsDetails", "Fetch the id and name of every team.", l, l.GetTeamsDetails, NoParams{})
	l.Toolkit.Register("GetIssueDetails", "Retrieve id, title and description of an issue by ID.", l, l.GetIssueDetails, IssueParams{})
	l.Toolkit.Register("CreateIssue", "Create a new issue within a team (and optionally a project / assignee).", l, l.CreateIssue, CreateIssueParams{})
	l.Toolkit.Register("UpdateIssue", "Update the title of an issue by ID.", l, l.UpdateIssue, UpdateIssueParams{})
	l.Toolkit.Register("GetUserAssignedIssues", "Retrieve issues assigned to a user by user ID.", l, l.GetUserAssignedIssues, UserParams{})
	l.Toolkit.Register("GetWorkflowIssues", "Retrieve issues within a workflow state by workflow state ID.", l, l.GetWorkflowIssues, WorkflowParams{})
	l.Toolkit.Register("GetHighPriorityIssues", "Retrieve issues with a high priority (priority <= 2).", l, l.GetHighPriorityIssues, NoParams{})
	return l
}

// query executes a GraphQL operation and returns its "data" object.
func (l *LinearTools) query(q string, variables map[string]interface{}) (map[string]interface{}, error) {
	if l.apiKey == "" {
		return nil, fmt.Errorf("LINEAR_API_KEY is required")
	}
	var resp struct {
		Data   map[string]interface{} `json:"data"`
		Errors json.RawMessage        `json:"errors"`
	}
	body := map[string]interface{}{"query": q, "variables": variables}
	if err := l.client.Into(http.MethodPost, "", nil, body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Errors) > 0 && string(resp.Errors) != "null" {
		return nil, fmt.Errorf("GraphQL Error: %s", string(resp.Errors))
	}
	if resp.Data == nil {
		resp.Data = map[string]interface{}{}
	}
	return resp.Data, nil
}

func nodes(m map[string]interface{}, keys ...string) interface{} {
	cur := m
	for _, k := range keys {
		cur = restkit.Map(cur, k)
	}
	if v, ok := cur["nodes"]; ok && v != nil {
		return v
	}
	return []interface{}{}
}

// GetUserDetails returns the authenticated user.
func (l *LinearTools) GetUserDetails(NoParams) (interface{}, error) {
	data, err := l.query(`query Me { viewer { id name email } }`, nil)
	if err != nil {
		return nil, err
	}
	if v := restkit.Map(data, "viewer"); v != nil {
		return restkit.CompactJSON(v), nil
	}
	return "Failed to retrieve the current user details", nil
}

// GetTeamsDetails returns all teams.
func (l *LinearTools) GetTeamsDetails(NoParams) (interface{}, error) {
	data, err := l.query(`query Teams { teams { nodes { id name } } }`, nil)
	if err != nil {
		return nil, err
	}
	if restkit.Map(data, "teams") != nil {
		return restkit.CompactJSON(nodes(data, "teams")), nil
	}
	return "Failed to retrieve the teams details", nil
}

// GetIssueDetails returns an issue.
func (l *LinearTools) GetIssueDetails(params IssueParams) (interface{}, error) {
	data, err := l.query(`query IssueDetails ($issueId: String!) { issue(id: $issueId) { id title description } }`,
		map[string]interface{}{"issueId": params.IssueID})
	if err != nil {
		return nil, err
	}
	if v := restkit.Map(data, "issue"); v != nil {
		return restkit.CompactJSON(v), nil
	}
	return fmt.Sprintf("Failed to retrieve issue with ID %s.", params.IssueID), nil
}

// CreateIssue creates an issue.
func (l *LinearTools) CreateIssue(params CreateIssueParams) (interface{}, error) {
	q := `mutation IssueCreate ($title: String!, $description: String!, $teamId: String!, $projectId: String, $assigneeId: String) {
  issueCreate(input: { title: $title, description: $description, teamId: $teamId, projectId: $projectId, assigneeId: $assigneeId }) {
    success
    issue { id title url }
  }
}`
	vars := map[string]interface{}{"title": params.Title, "description": params.Description, "teamId": params.TeamID}
	if params.ProjectID != "" {
		vars["projectId"] = params.ProjectID
	}
	if params.AssigneeID != "" {
		vars["assigneeId"] = params.AssigneeID
	}
	data, err := l.query(q, vars)
	if err != nil {
		return nil, err
	}
	res := restkit.Map(data, "issueCreate")
	if ok, _ := res["success"].(bool); ok {
		return restkit.CompactJSON(restkit.Map(res, "issue")), nil
	}
	return "Issue creation failed.", nil
}

// UpdateIssue updates the title of an issue.
func (l *LinearTools) UpdateIssue(params UpdateIssueParams) (interface{}, error) {
	q := `mutation IssueUpdate ($issueId: String!, $title: String!) {
  issueUpdate(id: $issueId, input: { title: $title }) {
    success
    issue { id title state { id name } }
  }
}`
	data, err := l.query(q, map[string]interface{}{"issueId": params.IssueID, "title": params.Title})
	if err != nil {
		return nil, err
	}
	res := restkit.Map(data, "issueUpdate")
	if ok, _ := res["success"].(bool); ok {
		return restkit.CompactJSON(restkit.Map(res, "issue")), nil
	}
	return fmt.Sprintf("Failed to update issue ID %s. Success flag was false.", params.IssueID), nil
}

// GetUserAssignedIssues returns issues assigned to a user.
func (l *LinearTools) GetUserAssignedIssues(params UserParams) (interface{}, error) {
	data, err := l.query(`query UserAssignedIssues($userId: String!) { user(id: $userId) { id name assignedIssues { nodes { id title } } } }`,
		map[string]interface{}{"userId": params.UserID})
	if err != nil {
		return nil, err
	}
	if restkit.Map(data, "user") != nil {
		return restkit.CompactJSON(nodes(data, "user", "assignedIssues")), nil
	}
	return "Failed to retrieve user or issues.", nil
}

// GetWorkflowIssues returns the issues in a workflow state.
func (l *LinearTools) GetWorkflowIssues(params WorkflowParams) (interface{}, error) {
	data, err := l.query(`query WorkflowStateIssues($workflowId: String!) { workflowState(id: $workflowId) { issues { nodes { title } } } }`,
		map[string]interface{}{"workflowId": params.WorkflowID})
	if err != nil {
		return nil, err
	}
	if restkit.Map(data, "workflowState") != nil {
		return restkit.CompactJSON(nodes(data, "workflowState", "issues")), nil
	}
	return "Failed to retrieve issues for the specified workflow state.", nil
}

// GetHighPriorityIssues returns issues with priority <= 2.
func (l *LinearTools) GetHighPriorityIssues(NoParams) (interface{}, error) {
	data, err := l.query(`query HighPriorityIssues { issues(filter: { priority: { lte: 2 } }) { nodes { id title priority } } }`, nil)
	if err != nil {
		return nil, err
	}
	if restkit.Map(data, "issues") != nil {
		return restkit.CompactJSON(nodes(data, "issues")), nil
	}
	return "Failed to retrieve high-priority issues.", nil
}

// Execute implements toolkit.Tool.
func (l *LinearTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return l.Toolkit.Execute(methodName, input)
}

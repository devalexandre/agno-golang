// Package gitlab ports Agno's GitlabTools: read-only access to GitLab
// projects, merge requests and issues through the GitLab REST API v4
// (PRIVATE-TOKEN authentication).
package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the GitLab instance used when none is configured.
const DefaultBaseURL = "https://gitlab.com"

// GitlabTools queries a GitLab instance.
type GitlabTools struct {
	toolkit.Toolkit
	accessToken string
	baseURL     string
	httpClient  *http.Client
	client      *restkit.Client

	enableListProjects      bool
	enableGetProjects       bool
	enableListMergeRequests bool
	enableGetMergeRequest   bool
	enableListIssues        bool
}

// Option configures GitlabTools.
type Option func(*GitlabTools)

// WithAccessToken sets the personal access token (default: GITLAB_ACCESS_TOKEN).
func WithAccessToken(token string) Option { return func(g *GitlabTools) { g.accessToken = token } }

// WithBaseURL sets the GitLab instance URL (default: GITLAB_BASE_URL or https://gitlab.com).
func WithBaseURL(u string) Option { return func(g *GitlabTools) { g.baseURL = u } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(g *GitlabTools) { g.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(g *GitlabTools) { g.httpClient = c } }

// WithEnableListProjects toggles ListProjects (default true).
func WithEnableListProjects(b bool) Option { return func(g *GitlabTools) { g.enableListProjects = b } }

// WithEnableGetProjects toggles GetProject (default true).
func WithEnableGetProjects(b bool) Option { return func(g *GitlabTools) { g.enableGetProjects = b } }

// WithEnableListMergeRequests toggles ListMergeRequests (default true).
func WithEnableListMergeRequests(b bool) Option {
	return func(g *GitlabTools) { g.enableListMergeRequests = b }
}

// WithEnableGetMergeRequest toggles GetMergeRequest (default true).
func WithEnableGetMergeRequest(b bool) Option {
	return func(g *GitlabTools) { g.enableGetMergeRequest = b }
}

// WithEnableListIssues toggles ListIssues (default true).
func WithEnableListIssues(b bool) Option { return func(g *GitlabTools) { g.enableListIssues = b } }

// ListProjectsParams are the parameters of ListProjects.
type ListProjectsParams struct {
	Search     string `json:"search,omitempty" description:"Optional search query for project name."`
	Owned      bool   `json:"owned,omitempty" description:"If true, return only projects owned by the current user."`
	Membership bool   `json:"membership,omitempty" description:"If true, return only projects the user is a member of."`
	Page       int    `json:"page,omitempty" description:"Page number for pagination. Default is 1."`
	PerPage    int    `json:"per_page,omitempty" description:"Items per page (max 100). Default is 20."`
}

// GetProjectParams are the parameters of GetProject.
type GetProjectParams struct {
	ProjectIDOrPath string `json:"project_id_or_path" description:"GitLab project ID or path (e.g. group/project)." required:"true"`
}

// ListMergeRequestsParams are the parameters of ListMergeRequests.
type ListMergeRequestsParams struct {
	ProjectIDOrPath string `json:"project_id_or_path" description:"GitLab project ID or path (e.g. group/project)." required:"true"`
	State           string `json:"state,omitempty" description:"Merge request state (opened, closed, merged, all). Default is opened."`
	SourceBranch    string `json:"source_branch,omitempty" description:"Optional source branch filter."`
	TargetBranch    string `json:"target_branch,omitempty" description:"Optional target branch filter."`
	AuthorUsername  string `json:"author_username,omitempty" description:"Optional author username filter."`
	Page            int    `json:"page,omitempty" description:"Page number for pagination. Default is 1."`
	PerPage         int    `json:"per_page,omitempty" description:"Items per page (max 100). Default is 20."`
}

// GetMergeRequestParams are the parameters of GetMergeRequest.
type GetMergeRequestParams struct {
	ProjectIDOrPath string `json:"project_id_or_path" description:"GitLab project ID or path (e.g. group/project)." required:"true"`
	MergeRequestIID int    `json:"merge_request_iid" description:"Internal merge request IID in the project." required:"true"`
}

// ListIssuesParams are the parameters of ListIssues.
type ListIssuesParams struct {
	ProjectIDOrPath  string `json:"project_id_or_path" description:"GitLab project ID or path (e.g. group/project)." required:"true"`
	State            string `json:"state,omitempty" description:"Issue state (opened, closed, all). Default is opened."`
	Labels           string `json:"labels,omitempty" description:"Optional comma-separated label filter."`
	AuthorUsername   string `json:"author_username,omitempty" description:"Optional author username filter."`
	AssigneeUsername string `json:"assignee_username,omitempty" description:"Optional assignee username filter."`
	Search           string `json:"search,omitempty" description:"Optional full-text search query."`
	Page             int    `json:"page,omitempty" description:"Page number for pagination. Default is 1."`
	PerPage          int    `json:"per_page,omitempty" description:"Items per page (max 100). Default is 20."`
}

// New creates the GitLab toolkit.
func New(opts ...Option) *GitlabTools {
	g := &GitlabTools{
		enableListProjects:      true,
		enableGetProjects:       true,
		enableListMergeRequests: true,
		enableGetMergeRequest:   true,
		enableListIssues:        true,
	}
	g.Toolkit = toolkit.NewToolkit()
	g.Toolkit.Name = "gitlab"
	g.Toolkit.Description = "Read GitLab projects, merge requests and issues."
	for _, o := range opts {
		o(g)
	}
	g.accessToken = restkit.Env(g.accessToken, "GITLAB_ACCESS_TOKEN")
	g.baseURL = strings.TrimRight(restkit.Env(g.baseURL, "GITLAB_BASE_URL"), "/")
	if g.baseURL == "" {
		g.baseURL = DefaultBaseURL
	}
	if g.httpClient == nil {
		g.httpClient = g.Toolkit.HTTPClient()
	}
	headers := map[string]string{}
	if g.accessToken != "" {
		headers["PRIVATE-TOKEN"] = g.accessToken
	}
	g.client = &restkit.Client{HTTP: g.httpClient, BaseURL: g.baseURL + "/api/v4", Name: "GitLab API", Headers: headers}

	if g.enableListProjects {
		g.Toolkit.Register("ListProjects", "List projects visible to the authenticated user. Returns JSON with data and pagination metadata.", g, g.ListProjects, ListProjectsParams{})
	}
	if g.enableGetProjects {
		g.Toolkit.Register("GetProject", "Get details for a single GitLab project.", g, g.GetProject, GetProjectParams{})
	}
	if g.enableListMergeRequests {
		g.Toolkit.Register("ListMergeRequests", "List merge requests for a GitLab project. Returns JSON with data and pagination metadata.", g, g.ListMergeRequests, ListMergeRequestsParams{})
	}
	if g.enableGetMergeRequest {
		g.Toolkit.Register("GetMergeRequest", "Get details for a single merge request in a GitLab project.", g, g.GetMergeRequest, GetMergeRequestParams{})
	}
	if g.enableListIssues {
		g.Toolkit.Register("ListIssues", "List issues for a GitLab project. Returns JSON with data and pagination metadata.", g, g.ListIssues, ListIssuesParams{})
	}
	return g
}

func boundPageSize(perPage int) int {
	if perPage == 0 {
		perPage = 20
	}
	if perPage < 1 {
		return 1
	}
	if perPage > 100 {
		return 100
	}
	return perPage
}

func projectRef(p string) string { return url.PathEscape(p) }

// errorMessage mirrors Python's _http_error_message: "<status>: <message>".
func errorMessage(err error) string {
	var apiErr *restkit.APIError
	if errors.As(err, &apiErr) {
		detail := ""
		switch v := restkit.Decode([]byte(apiErr.Body)).(type) {
		case map[string]interface{}:
			msg := v["message"]
			if msg == nil {
				msg = v["error"]
			}
			switch m := msg.(type) {
			case nil:
			case string:
				detail = m
			default:
				detail = restkit.CompactJSON(m)
			}
		case []interface{}:
			detail = restkit.CompactJSON(v)
		}
		if detail == "" {
			detail = strings.TrimSpace(apiErr.Body)
		}
		if detail == "" {
			detail = http.StatusText(apiErr.Status)
		}
		return fmt.Sprintf("%d: %s", apiErr.Status, detail)
	}
	return err.Error()
}

func (g *GitlabTools) get(path string, q url.Values) (interface{}, string) {
	v, err := g.client.JSON(http.MethodGet, path, q, nil)
	if err != nil {
		return nil, restkit.ErrorJSON(errorMessage(err))
	}
	return v, ""
}

func username(v interface{}) interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m["username"]
	}
	return nil
}

func pick(m map[string]interface{}, keys ...string) map[string]interface{} {
	out := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		out[k] = m[k]
	}
	return out
}

func serializeProject(m map[string]interface{}) map[string]interface{} {
	return pick(m, "id", "name", "path", "path_with_namespace", "description", "web_url", "default_branch", "visibility", "archived", "last_activity_at")
}

func serializeMergeRequest(m map[string]interface{}) map[string]interface{} {
	out := pick(m, "id", "iid", "title", "description", "state", "web_url", "source_branch", "target_branch", "created_at", "updated_at", "merged_at")
	out["author"] = username(m["author"])
	return out
}

func serializeIssue(m map[string]interface{}) map[string]interface{} {
	out := pick(m, "id", "iid", "title", "description", "state", "web_url", "created_at", "updated_at", "due_date")
	labels := m["labels"]
	if labels == nil {
		labels = []interface{}{}
	}
	out["labels"] = labels
	out["author"] = username(m["author"])
	assignees := []interface{}{}
	if arr, ok := m["assignees"].([]interface{}); ok {
		for _, a := range arr {
			if am, ok := a.(map[string]interface{}); ok {
				assignees = append(assignees, am["username"])
			}
		}
	}
	out["assignees"] = assignees
	return out
}

func listResult(v interface{}, page, perPage int, ser func(map[string]interface{}) map[string]interface{}) string {
	data := []map[string]interface{}{}
	if arr, ok := v.([]interface{}); ok {
		for _, it := range arr {
			if m, ok := it.(map[string]interface{}); ok {
				data = append(data, ser(m))
			}
		}
	}
	return restkit.JSONString(map[string]interface{}{
		"data": data,
		"meta": map[string]int{"current_page": page, "per_page": perPage, "returned_items": len(data)},
	})
}

func pageQuery(page, perPage int) (int, int, url.Values) {
	if page <= 0 {
		page = 1
	}
	perPage = boundPageSize(perPage)
	return page, perPage, url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(perPage)}}
}

// ListProjects lists projects visible to the authenticated user.
func (g *GitlabTools) ListProjects(params ListProjectsParams) (interface{}, error) {
	page, perPage, q := pageQuery(params.Page, params.PerPage)
	q.Set("owned", strconv.FormatBool(params.Owned))
	q.Set("membership", strconv.FormatBool(params.Membership))
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	v, errStr := g.get("/projects", q)
	if errStr != "" {
		return errStr, nil
	}
	return listResult(v, page, perPage, serializeProject), nil
}

// GetProject returns details for a single project.
func (g *GitlabTools) GetProject(params GetProjectParams) (interface{}, error) {
	if params.ProjectIDOrPath == "" {
		return restkit.ErrorJSON("project_id_or_path is required"), nil
	}
	v, errStr := g.get("/projects/"+projectRef(params.ProjectIDOrPath), nil)
	if errStr != "" {
		return errStr, nil
	}
	m, _ := v.(map[string]interface{})
	return restkit.JSONString(serializeProject(m)), nil
}

// ListMergeRequests lists merge requests for a project.
func (g *GitlabTools) ListMergeRequests(params ListMergeRequestsParams) (interface{}, error) {
	if params.ProjectIDOrPath == "" {
		return restkit.ErrorJSON("project_id_or_path is required"), nil
	}
	page, perPage, q := pageQuery(params.Page, params.PerPage)
	state := params.State
	if state == "" {
		state = "opened"
	}
	q.Set("state", state)
	if params.SourceBranch != "" {
		q.Set("source_branch", params.SourceBranch)
	}
	if params.TargetBranch != "" {
		q.Set("target_branch", params.TargetBranch)
	}
	if params.AuthorUsername != "" {
		q.Set("author_username", params.AuthorUsername)
	}
	v, errStr := g.get("/projects/"+projectRef(params.ProjectIDOrPath)+"/merge_requests", q)
	if errStr != "" {
		return errStr, nil
	}
	return listResult(v, page, perPage, serializeMergeRequest), nil
}

// GetMergeRequest returns a single merge request.
func (g *GitlabTools) GetMergeRequest(params GetMergeRequestParams) (interface{}, error) {
	if params.ProjectIDOrPath == "" {
		return restkit.ErrorJSON("project_id_or_path is required"), nil
	}
	path := fmt.Sprintf("/projects/%s/merge_requests/%d", projectRef(params.ProjectIDOrPath), params.MergeRequestIID)
	v, errStr := g.get(path, nil)
	if errStr != "" {
		return errStr, nil
	}
	m, _ := v.(map[string]interface{})
	return restkit.JSONString(serializeMergeRequest(m)), nil
}

// ListIssues lists issues for a project.
func (g *GitlabTools) ListIssues(params ListIssuesParams) (interface{}, error) {
	if params.ProjectIDOrPath == "" {
		return restkit.ErrorJSON("project_id_or_path is required"), nil
	}
	page, perPage, q := pageQuery(params.Page, params.PerPage)
	state := params.State
	if state == "" {
		state = "opened"
	}
	q.Set("state", state)
	if params.Labels != "" {
		q.Set("labels", params.Labels)
	}
	if params.AuthorUsername != "" {
		q.Set("author_username", params.AuthorUsername)
	}
	if params.AssigneeUsername != "" {
		q.Set("assignee_username", params.AssigneeUsername)
	}
	if params.Search != "" {
		q.Set("search", params.Search)
	}
	v, errStr := g.get("/projects/"+projectRef(params.ProjectIDOrPath)+"/issues", q)
	if errStr != "" {
		return errStr, nil
	}
	return listResult(v, page, perPage, serializeIssue), nil
}

// Execute implements toolkit.Tool.
func (g *GitlabTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return g.Toolkit.Execute(methodName, input)
}

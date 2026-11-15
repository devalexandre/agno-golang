// Package clickup ports Agno's ClickUpTools: spaces, lists and tasks of a
// ClickUp workspace through the ClickUp REST API v2 (personal API token in
// the Authorization header).
package clickup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the ClickUp API base URL.
const DefaultBaseURL = "https://api.clickup.com/api/v2"

// ClickUpTools manages ClickUp tasks.
type ClickUpTools struct {
	toolkit.Toolkit
	apiKey        string
	masterSpaceID string
	baseURL       string
	httpClient    *http.Client
	client        *restkit.Client
}

// Option configures ClickUpTools.
type Option func(*ClickUpTools)

// WithAPIKey sets the API token (default: CLICKUP_API_KEY).
func WithAPIKey(key string) Option { return func(c *ClickUpTools) { c.apiKey = key } }

// WithMasterSpaceID sets the workspace (team) ID (default: MASTER_SPACE_ID).
func WithMasterSpaceID(id string) Option { return func(c *ClickUpTools) { c.masterSpaceID = id } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(c *ClickUpTools) { c.baseURL = u } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(c *ClickUpTools) { c.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *ClickUpTools) { c.httpClient = h } }

// NoParams is used by methods without arguments.
type NoParams struct{}

// SpaceParams identify a space by name.
type SpaceParams struct {
	SpaceName string `json:"space_name" description:"Name of the space (exact, case-insensitive, or a regex pattern)." required:"true"`
}

// CreateTaskParams are the parameters of CreateTask.
type CreateTaskParams struct {
	SpaceName       string `json:"space_name" description:"Name of the space to create the task in." required:"true"`
	TaskName        string `json:"task_name" description:"Name of the task." required:"true"`
	TaskDescription string `json:"task_description" description:"Description of the task." required:"true"`
}

// TaskParams identify a task.
type TaskParams struct {
	TaskID string `json:"task_id" description:"The ID of the task." required:"true"`
}

// UpdateTaskParams are the parameters of UpdateTask. Only non-empty fields are sent.
type UpdateTaskParams struct {
	TaskID      string `json:"task_id" description:"The ID of the task." required:"true"`
	Name        string `json:"name,omitempty" description:"New task name."`
	Description string `json:"description,omitempty" description:"New task description."`
	Status      string `json:"status,omitempty" description:"New task status (e.g. 'to do', 'in progress', 'complete')."`
	Priority    int    `json:"priority,omitempty" description:"New priority: 1 urgent, 2 high, 3 normal, 4 low."`
	DueDate     int64  `json:"due_date,omitempty" description:"Due date as Unix time in milliseconds."`
}

// New creates the ClickUp toolkit.
func New(opts ...Option) *ClickUpTools {
	c := &ClickUpTools{baseURL: DefaultBaseURL}
	c.Toolkit = toolkit.NewToolkit()
	c.Toolkit.Name = "clickup"
	c.Toolkit.Description = "Manage ClickUp spaces, lists and tasks."
	for _, o := range opts {
		o(c)
	}
	c.apiKey = restkit.Env(c.apiKey, "CLICKUP_API_KEY")
	c.masterSpaceID = restkit.Env(c.masterSpaceID, "MASTER_SPACE_ID")
	if c.httpClient == nil {
		c.httpClient = c.Toolkit.HTTPClient()
	}
	c.client = &restkit.Client{HTTP: c.httpClient, BaseURL: c.baseURL, Name: "ClickUp API", Headers: map[string]string{"Authorization": c.apiKey}}

	c.Toolkit.Register("ListTasks", "List all tasks in a space (across all its lists).", c, c.ListTasks, SpaceParams{})
	c.Toolkit.Register("CreateTask", "Create a new task in the first list of a space.", c, c.CreateTask, CreateTaskParams{})
	c.Toolkit.Register("GetTask", "Get details of a specific task.", c, c.GetTask, TaskParams{})
	c.Toolkit.Register("UpdateTask", "Update a task (name, description, status, priority, due_date).", c, c.UpdateTask, UpdateTaskParams{})
	c.Toolkit.Register("DeleteTask", "Delete a specific task.", c, c.DeleteTask, TaskParams{})
	c.Toolkit.Register("ListSpaces", "List all spaces in the workspace.", c, c.ListSpaces, NoParams{})
	c.Toolkit.Register("ListLists", "List all lists in a space.", c, c.ListLists, SpaceParams{})
	return c
}

func errMap(msg string) map[string]interface{} { return map[string]interface{}{"error": msg} }

// request mirrors Python's _make_request: failures become {"error": ...}.
func (c *ClickUpTools) request(method, endpoint string, body interface{}) map[string]interface{} {
	if c.apiKey == "" {
		return errMap("CLICKUP_API_KEY not set. Please set the CLICKUP_API_KEY environment variable.")
	}
	if c.masterSpaceID == "" {
		return errMap("MASTER_SPACE_ID not set. Please set the MASTER_SPACE_ID environment variable.")
	}
	var out map[string]interface{}
	if err := c.client.Into(method, endpoint, nil, body, &out); err != nil {
		return errMap(err.Error())
	}
	if out == nil {
		out = map[string]interface{}{}
	}
	return out
}

func hasError(m map[string]interface{}) bool { _, ok := m["error"]; return ok }

func findByName(items []map[string]interface{}, name string) map[string]interface{} {
	if name == "" {
		if len(items) > 0 {
			return items[0]
		}
		return nil
	}
	re, reErr := regexp.Compile("(?i)" + name)
	for _, it := range items {
		n := restkit.Str(it, "name")
		if strings.EqualFold(n, name) {
			return it
		}
		if reErr == nil && re.MatchString(n) {
			return it
		}
		if reErr != nil && strings.Contains(strings.ToLower(n), strings.ToLower(name)) {
			return it
		}
	}
	return nil
}

func (c *ClickUpTools) getSpace(spaceName string) map[string]interface{} {
	spaces := c.request(http.MethodGet, "team/"+url.PathEscape(c.masterSpaceID)+"/space", nil)
	if hasError(spaces) {
		return spaces
	}
	list := restkit.Slice(spaces, "spaces")
	if len(list) == 0 {
		return errMap("No spaces found")
	}
	space := findByName(list, spaceName)
	if space == nil {
		return errMap(fmt.Sprintf("Space '%s' not found", spaceName))
	}
	return space
}

func (c *ClickUpTools) spaceLists(spaceID string) map[string]interface{} {
	return c.request(http.MethodGet, "space/"+url.PathEscape(spaceID)+"/list", nil)
}

// ListTasks lists every task of every list in a space.
func (c *ClickUpTools) ListTasks(params SpaceParams) (interface{}, error) {
	space := c.getSpace(params.SpaceName)
	if hasError(space) {
		return restkit.JSONString(space), nil
	}
	lists := restkit.Slice(c.spaceLists(restkit.Str(space, "id")), "lists")
	if len(lists) == 0 {
		return restkit.JSONString(errMap(fmt.Sprintf("No lists found in space '%s'", params.SpaceName))), nil
	}
	all := []map[string]interface{}{}
	for _, l := range lists {
		tasks := c.request(http.MethodGet, "list/"+url.PathEscape(restkit.Str(l, "id"))+"/task", nil)
		if hasError(tasks) {
			continue
		}
		for _, t := range restkit.Slice(tasks, "tasks") {
			t["list_name"] = l["name"]
			all = append(all, t)
		}
	}
	return restkit.JSONString(map[string]interface{}{"tasks": all}), nil
}

// CreateTask creates a task in the first list of a space.
func (c *ClickUpTools) CreateTask(params CreateTaskParams) (interface{}, error) {
	space := c.getSpace(params.SpaceName)
	if hasError(space) {
		return restkit.JSONString(space), nil
	}
	lists := restkit.Slice(c.spaceLists(restkit.Str(space, "id")), "lists")
	if len(lists) == 0 {
		return restkit.JSONString(errMap(fmt.Sprintf("No lists found in space '%s'", params.SpaceName))), nil
	}
	body := map[string]interface{}{"name": params.TaskName, "description": params.TaskDescription}
	task := c.request(http.MethodPost, "list/"+url.PathEscape(restkit.Str(lists[0], "id"))+"/task", body)
	return restkit.JSONString(task), nil
}

// ListSpaces lists the spaces of the workspace.
func (c *ClickUpTools) ListSpaces(NoParams) (interface{}, error) {
	return restkit.JSONString(c.request(http.MethodGet, "team/"+url.PathEscape(c.masterSpaceID)+"/space", nil)), nil
}

// ListLists lists the lists of a space.
func (c *ClickUpTools) ListLists(params SpaceParams) (interface{}, error) {
	space := c.getSpace(params.SpaceName)
	if hasError(space) {
		return restkit.JSONString(space), nil
	}
	return restkit.JSONString(c.spaceLists(restkit.Str(space, "id"))), nil
}

// GetTask returns a task.
func (c *ClickUpTools) GetTask(params TaskParams) (interface{}, error) {
	return restkit.JSONString(c.request(http.MethodGet, "task/"+url.PathEscape(params.TaskID), nil)), nil
}

// UpdateTask updates a task.
func (c *ClickUpTools) UpdateTask(params UpdateTaskParams) (interface{}, error) {
	body := map[string]interface{}{}
	if params.Name != "" {
		body["name"] = params.Name
	}
	if params.Description != "" {
		body["description"] = params.Description
	}
	if params.Status != "" {
		body["status"] = params.Status
	}
	if params.Priority != 0 {
		body["priority"] = params.Priority
	}
	if params.DueDate != 0 {
		body["due_date"] = params.DueDate
	}
	return restkit.JSONString(c.request(http.MethodPut, "task/"+url.PathEscape(params.TaskID), body)), nil
}

// DeleteTask deletes a task.
func (c *ClickUpTools) DeleteTask(params TaskParams) (interface{}, error) {
	result := c.request(http.MethodDelete, "task/"+url.PathEscape(params.TaskID), nil)
	if !hasError(result) {
		result = map[string]interface{}{"success": true, "message": fmt.Sprintf("Task %s deleted successfully", params.TaskID)}
	}
	return restkit.JSONString(result), nil
}

// Execute implements toolkit.Tool.
func (c *ClickUpTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return c.Toolkit.Execute(methodName, input)
}

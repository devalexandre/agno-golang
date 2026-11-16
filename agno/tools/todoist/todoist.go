// Package todoist ports Agno's TodoistTools: create, read, update, close and
// delete Todoist tasks and list projects through the Todoist API v1 (the API
// targeted by todoist-api-python v3) with a Bearer token.
package todoist

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Todoist API base URL.
const DefaultBaseURL = "https://api.todoist.com/api/v1"

// TodoistTools manages Todoist tasks and projects.
type TodoistTools struct {
	toolkit.Toolkit
	apiToken   string
	baseURL    string
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures TodoistTools.
type Option func(*TodoistTools)

// WithAPIToken sets the API token (default: TODOIST_API_TOKEN).
func WithAPIToken(token string) Option { return func(t *TodoistTools) { t.apiToken = token } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *TodoistTools) { t.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(t *TodoistTools) { t.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *TodoistTools) { t.httpClient = c } }

// NoParams is used by methods without arguments.
type NoParams struct{}

// CreateTaskParams are the parameters of CreateTask.
type CreateTaskParams struct {
	Content   string   `json:"content" description:"The task content/description." required:"true"`
	ProjectID string   `json:"project_id,omitempty" description:"Optional ID of the project to add the task to."`
	DueString string   `json:"due_string,omitempty" description:"Optional due date in natural language (e.g., 'tomorrow at 12:00')."`
	Priority  int      `json:"priority,omitempty" description:"Optional priority level (1-4, where 4 is highest)."`
	Labels    []string `json:"labels,omitempty" description:"Optional list of label names to apply to the task."`
}

// TaskParams identify a task.
type TaskParams struct {
	TaskID string `json:"task_id" description:"The ID of the task." required:"true"`
}

// UpdateTaskParams are the parameters of UpdateTask. Only non-empty fields are sent.
type UpdateTaskParams struct {
	TaskID      string   `json:"task_id" description:"The ID of the task to update." required:"true"`
	Content     string   `json:"content,omitempty" description:"The task content/name."`
	Description string   `json:"description,omitempty" description:"The task description."`
	Labels      []string `json:"labels,omitempty" description:"Array of label names."`
	Priority    int      `json:"priority,omitempty" description:"Task priority from 1 (normal) to 4 (urgent)."`
	DueString   string   `json:"due_string,omitempty" description:"Human readable date ('next Monday', 'tomorrow', etc)."`
	DueDate     string   `json:"due_date,omitempty" description:"Specific date in YYYY-MM-DD format."`
	DueDatetime string   `json:"due_datetime,omitempty" description:"Specific date and time in RFC3339 format."`
	DueLang     string   `json:"due_lang,omitempty" description:"2-letter code specifying language of due_string ('en', 'fr', etc)."`
	AssigneeID  string   `json:"assignee_id,omitempty" description:"The responsible user ID."`
	SectionID   string   `json:"section_id,omitempty" description:"ID of the section to move task to."`
}

// New creates the Todoist toolkit.
func New(opts ...Option) *TodoistTools {
	t := &TodoistTools{baseURL: DefaultBaseURL}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "todoist"
	t.Toolkit.Description = "A toolkit for interacting with Todoist tasks and projects."
	for _, o := range opts {
		o(t)
	}
	t.apiToken = restkit.Env(t.apiToken, "TODOIST_API_TOKEN")
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	t.client = &restkit.Client{
		HTTP:    t.httpClient,
		BaseURL: t.baseURL,
		Name:    "Todoist API",
		Headers: map[string]string{"Authorization": "Bearer " + t.apiToken},
	}
	t.Toolkit.Register("CreateTask", "Create a new task in Todoist.", t, t.CreateTask, CreateTaskParams{})
	t.Toolkit.Register("GetTask", "Get a specific task by ID.", t, t.GetTask, TaskParams{})
	t.Toolkit.Register("UpdateTask", "Update an existing task with the specified parameters.", t, t.UpdateTask, UpdateTaskParams{})
	t.Toolkit.Register("CloseTask", "Mark a task as completed.", t, t.CloseTask, TaskParams{})
	t.Toolkit.Register("DeleteTask", "Delete a task.", t, t.DeleteTask, TaskParams{})
	t.Toolkit.Register("GetActiveTasks", "Get all active (not completed) tasks.", t, t.GetActiveTasks, NoParams{})
	t.Toolkit.Register("GetProjects", "Get all projects.", t, t.GetProjects, NoParams{})
	return t
}

func (t *TodoistTools) check() error {
	if t.apiToken == "" {
		return fmt.Errorf("TODOIST_API_TOKEN not set. Please set the TODOIST_API_TOKEN environment variable")
	}
	return nil
}

func first(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

// taskToDict mirrors Python's _task_to_dict over the raw API object.
func taskToDict(task map[string]interface{}) map[string]interface{} {
	id := restkit.Str(task, "id")
	taskURL := first(task, "url")
	if taskURL == nil && id != "" {
		taskURL = "https://app.todoist.com/app/task/" + id
	}
	labels := task["labels"]
	if labels == nil {
		labels = []interface{}{}
	}
	d := map[string]interface{}{
		"id":          task["id"],
		"content":     task["content"],
		"description": task["description"],
		"project_id":  task["project_id"],
		"section_id":  task["section_id"],
		"parent_id":   task["parent_id"],
		"order":       first(task, "child_order", "order"),
		"priority":    task["priority"],
		"url":         taskURL,
		"creator_id":  first(task, "added_by_uid", "creator_id"),
		"created_at":  first(task, "added_at", "created_at"),
		"labels":      labels,
	}
	if c := first(task, "note_count", "comment_count"); c != nil {
		d["comment_count"] = c
	}
	if due := restkit.Map(task, "due"); due != nil {
		d["due"] = map[string]interface{}{
			"date":         due["date"],
			"string":       due["string"],
			"lang":         due["lang"],
			"is_recurring": due["is_recurring"],
			"timezone":     due["timezone"],
		}
	}
	return d
}

// CreateTask creates a task and returns it as JSON.
func (t *TodoistTools) CreateTask(p CreateTaskParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	labels := p.Labels
	if labels == nil {
		labels = []string{}
	}
	body := map[string]interface{}{"content": p.Content, "labels": labels}
	if p.ProjectID != "" {
		body["project_id"] = p.ProjectID
	}
	if p.DueString != "" {
		body["due_string"] = p.DueString
	}
	if p.Priority != 0 {
		body["priority"] = p.Priority
	}
	var task map[string]interface{}
	if err := t.client.Into(http.MethodPost, "/tasks", nil, body, &task); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.CompactJSON(taskToDict(task)), nil
}

// GetTask returns a task by ID.
func (t *TodoistTools) GetTask(p TaskParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	var task map[string]interface{}
	if err := t.client.Into(http.MethodGet, "/tasks/"+url.PathEscape(p.TaskID), nil, nil, &task); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.CompactJSON(taskToDict(task)), nil
}

// UpdateTask updates only the provided fields and returns {"success": true}.
func (t *TodoistTools) UpdateTask(p UpdateTaskParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	set := func(k, v string) {
		if v != "" {
			updates[k] = v
		}
	}
	set("content", p.Content)
	set("description", p.Description)
	if p.Labels != nil {
		updates["labels"] = p.Labels
	}
	if p.Priority != 0 {
		updates["priority"] = p.Priority
	}
	set("due_string", p.DueString)
	set("due_date", p.DueDate)
	set("due_datetime", p.DueDatetime)
	set("due_lang", p.DueLang)
	set("assignee_id", p.AssigneeID)
	set("section_id", p.SectionID)
	if _, err := t.client.JSON(http.MethodPost, "/tasks/"+url.PathEscape(p.TaskID), nil, updates); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.CompactJSON(map[string]bool{"success": true}), nil
}

// CloseTask marks a task as completed.
func (t *TodoistTools) CloseTask(p TaskParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	if _, err := t.client.JSON(http.MethodPost, "/tasks/"+url.PathEscape(p.TaskID)+"/close", nil, nil); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.CompactJSON(map[string]bool{"success": true}), nil
}

// DeleteTask deletes a task.
func (t *TodoistTools) DeleteTask(p TaskParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	if _, err := t.client.JSON(http.MethodDelete, "/tasks/"+url.PathEscape(p.TaskID), nil, nil); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.CompactJSON(map[string]bool{"success": true}), nil
}

// results extracts the item list of a paginated ({"results": [...]}) or plain array response.
func results(v interface{}) []map[string]interface{} {
	switch r := v.(type) {
	case map[string]interface{}:
		return restkit.Slice(r, "results")
	case []interface{}:
		return restkit.Slice(map[string]interface{}{"r": r}, "r")
	}
	return []map[string]interface{}{}
}

// GetActiveTasks returns the first page of active tasks (as Python does).
func (t *TodoistTools) GetActiveTasks(NoParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	v, err := t.client.JSON(http.MethodGet, "/tasks", nil, nil)
	if err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	list := []map[string]interface{}{}
	for _, task := range results(v) {
		list = append(list, taskToDict(task))
	}
	return restkit.CompactJSON(list), nil
}

// GetProjects returns all projects as returned by the API.
func (t *TodoistTools) GetProjects(NoParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	all := []map[string]interface{}{}
	q := url.Values{}
	for page := 0; page < 100; page++ {
		v, err := t.client.JSON(http.MethodGet, "/projects", q, nil)
		if err != nil {
			return restkit.ErrorJSON(err.Error()), nil
		}
		all = append(all, results(v)...)
		m, _ := v.(map[string]interface{})
		cursor := restkit.Str(m, "next_cursor")
		if cursor == "" {
			break
		}
		q.Set("cursor", cursor)
	}
	return restkit.CompactJSON(all), nil
}

// Execute implements toolkit.Tool.
func (t *TodoistTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

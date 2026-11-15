// Package redmine ports Agno's RedmineTools: issues, comments, time entries,
// projects, users, memberships and versions through the Redmine JSON REST API
// (X-Redmine-API-Key header, or HTTP basic auth with username/password).
package redmine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// RedmineTools manages Redmine issues.
type RedmineTools struct {
	toolkit.Toolkit
	serverURL  string
	username   string
	password   string
	token      string
	enabled    map[string]bool
	all        bool
	httpClient *http.Client
	client     *restkit.Client
}

// Option configures RedmineTools.
type Option func(*RedmineTools)

// WithServerURL sets the Redmine server URL (default: REDMINE_SERVER_URL).
func WithServerURL(u string) Option { return func(r *RedmineTools) { r.serverURL = u } }

// WithBaseURL is an alias of WithServerURL.
func WithBaseURL(u string) Option { return WithServerURL(u) }

// WithUsername sets the username for basic auth (default: REDMINE_USERNAME).
func WithUsername(u string) Option { return func(r *RedmineTools) { r.username = u } }

// WithPassword sets the password for basic auth (default: REDMINE_PASSWORD).
func WithPassword(p string) Option { return func(r *RedmineTools) { r.password = p } }

// WithToken sets the API key (default: REDMINE_TOKEN). Takes precedence over basic auth.
func WithToken(t string) Option { return func(r *RedmineTools) { r.token = t } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(r *RedmineTools) { r.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(r *RedmineTools) { r.httpClient = c } }

// WithAll enables every method.
func WithAll() Option { return func(r *RedmineTools) { r.all = true } }

func enable(name string, b bool) Option { return func(r *RedmineTools) { r.enabled[name] = b } }

// WithEnableGetIssue toggles GetIssue (default true).
func WithEnableGetIssue(b bool) Option { return enable("GetIssue", b) }

// WithEnableCreateIssue toggles CreateIssue (default true).
func WithEnableCreateIssue(b bool) Option { return enable("CreateIssue", b) }

// WithEnableUpdateIssue toggles UpdateIssue (default true).
func WithEnableUpdateIssue(b bool) Option { return enable("UpdateIssue", b) }

// WithEnableSearchIssues toggles SearchIssues (default true).
func WithEnableSearchIssues(b bool) Option { return enable("SearchIssues", b) }

// WithEnableAddComment toggles AddComment (default true).
func WithEnableAddComment(b bool) Option { return enable("AddComment", b) }

// WithEnableLogTime toggles LogTime (default true).
func WithEnableLogTime(b bool) Option { return enable("LogTime", b) }

// WithEnableListProjects toggles ListProjects (default true).
func WithEnableListProjects(b bool) Option { return enable("ListProjects", b) }

// WithEnableListUsers toggles ListUsers (default true).
func WithEnableListUsers(b bool) Option { return enable("ListUsers", b) }

// WithEnableListProjectMembers toggles ListProjectMembers (default true).
func WithEnableListProjectMembers(b bool) Option { return enable("ListProjectMembers", b) }

// WithEnableListVersions toggles ListVersions (default true).
func WithEnableListVersions(b bool) Option { return enable("ListVersions", b) }

// NoParams is used by methods without arguments.
type NoParams struct{}

// GetIssueParams are the parameters of GetIssue.
type GetIssueParams struct {
	IssueID         string `json:"issue_id" description:"The numeric id of the issue to retrieve." required:"true"`
	IncludeComments bool   `json:"include_comments,omitempty" description:"Whether to include the issue's comments (journal notes). Defaults to False."`
}

// CreateIssueParams are the parameters of CreateIssue.
type CreateIssueParams struct {
	ProjectID      string  `json:"project_id" description:"The identifier of the project in which to create the issue." required:"true"`
	Subject        string  `json:"subject" description:"The subject (title) of the issue." required:"true"`
	Description    string  `json:"description" description:"The description of the issue." required:"true"`
	Tracker        string  `json:"tracker,omitempty" description:"The tracker name for the issue (e.g. 'Bug', 'Feature', 'Support'). The project default is used when omitted."`
	Priority       string  `json:"priority,omitempty" description:"The priority name for the issue (e.g. 'Low', 'Normal', 'High'). The project default is used when omitted."`
	AssignedToID   int     `json:"assigned_to_id,omitempty" description:"The id of the user to assign the issue to."`
	VersionID      int     `json:"version_id,omitempty" description:"The id of the target version (sprint/milestone). Use list_versions to resolve a version name to its id."`
	ParentIssueID  int     `json:"parent_issue_id,omitempty" description:"The id of the parent issue, to create this issue as a subtask."`
	EstimatedHours float64 `json:"estimated_hours,omitempty" description:"The estimated time to complete the issue, in hours."`
	StartDate      string  `json:"start_date,omitempty" description:"The start date in ISO format (YYYY-MM-DD)."`
	DueDate        string  `json:"due_date,omitempty" description:"The due date in ISO format (YYYY-MM-DD)."`
}

// UpdateIssueParams are the parameters of UpdateIssue. Only non-empty fields are changed.
type UpdateIssueParams struct {
	IssueID        string  `json:"issue_id" description:"The numeric id of the issue to update." required:"true"`
	Subject        string  `json:"subject,omitempty" description:"The new subject (title) of the issue."`
	Description    string  `json:"description,omitempty" description:"The new description of the issue."`
	Status         string  `json:"status,omitempty" description:"The new status name (e.g. 'In Progress', 'Resolved', 'Closed')."`
	Tracker        string  `json:"tracker,omitempty" description:"The new tracker name (e.g. 'Bug', 'Feature', 'Support')."`
	Priority       string  `json:"priority,omitempty" description:"The new priority name (e.g. 'Low', 'Normal', 'High')."`
	AssignedToID   int     `json:"assigned_to_id,omitempty" description:"The id of the user to assign the issue to."`
	DoneRatio      int     `json:"done_ratio,omitempty" description:"The completion percentage (0-100). Ignored for parent issues, whose value is computed from their subtasks."`
	VersionID      int     `json:"version_id,omitempty" description:"The id of the target version (sprint/milestone). Use list_versions to resolve a version name to its id."`
	ParentIssueID  int     `json:"parent_issue_id,omitempty" description:"The id of the parent issue, to make this issue a subtask."`
	EstimatedHours float64 `json:"estimated_hours,omitempty" description:"The estimated time to complete the issue, in hours."`
	StartDate      string  `json:"start_date,omitempty" description:"The start date in ISO format (YYYY-MM-DD)."`
	DueDate        string  `json:"due_date,omitempty" description:"The due date in ISO format (YYYY-MM-DD)."`
}

// SearchIssuesParams are the parameters of SearchIssues.
type SearchIssuesParams struct {
	Pattern      string `json:"pattern,omitempty" description:"Text to match against issue subjects."`
	ProjectID    string `json:"project_id,omitempty" description:"Restrict the search to this project identifier."`
	Status       string `json:"status,omitempty" description:"Filter by status: 'open', 'closed', 'all', or a status name (e.g. 'In Progress')."`
	Tracker      string `json:"tracker,omitempty" description:"Filter by tracker name (e.g. 'Bug', 'Feature', 'Support')."`
	AssignedToID int    `json:"assigned_to_id,omitempty" description:"Filter by the id of the assigned user."`
	MaxResults   int    `json:"max_results,omitempty" description:"Maximum number of results to return. Defaults to 50."`
}

// AddCommentParams are the parameters of AddComment.
type AddCommentParams struct {
	IssueID      string `json:"issue_id" description:"The numeric id of the issue." required:"true"`
	Comment      string `json:"comment" description:"The comment text." required:"true"`
	PrivateNotes bool   `json:"private_notes,omitempty" description:"Whether the comment is a private note visible only to users with permission. Defaults to False."`
}

// LogTimeParams are the parameters of LogTime.
type LogTimeParams struct {
	IssueID  string  `json:"issue_id" description:"The numeric id of the issue to log time against." required:"true"`
	Hours    float64 `json:"hours" description:"The number of hours spent. Must be greater than 0." required:"true"`
	Activity string  `json:"activity,omitempty" description:"The activity name (e.g. 'Design', 'Development'). Required unless the Redmine instance defines a default activity."`
	Comment  string  `json:"comment,omitempty" description:"A short description of the work done."`
	SpentOn  string  `json:"spent_on,omitempty" description:"The date the time was spent, in ISO format (YYYY-MM-DD). Defaults to today."`
}

// ProjectParams identify a project.
type ProjectParams struct {
	ProjectID string `json:"project_id" description:"The identifier of the project." required:"true"`
}

// New creates the Redmine toolkit.
func New(opts ...Option) *RedmineTools {
	r := &RedmineTools{enabled: map[string]bool{}}
	r.Toolkit = toolkit.NewToolkit()
	r.Toolkit.Name = "redmine_tools"
	r.Toolkit.Description = "Manage Redmine issues, comments, time entries, projects, users and versions."
	for _, o := range opts {
		o(r)
	}
	r.serverURL = strings.TrimRight(restkit.Env(r.serverURL, "REDMINE_SERVER_URL"), "/")
	r.username = restkit.Env(r.username, "REDMINE_USERNAME")
	r.password = restkit.Env(r.password, "REDMINE_PASSWORD")
	r.token = restkit.Env(r.token, "REDMINE_TOKEN")
	if r.httpClient == nil {
		r.httpClient = r.Toolkit.HTTPClient()
	}
	r.client = &restkit.Client{HTTP: r.httpClient, BaseURL: r.serverURL, Name: "Redmine API"}
	if r.token != "" {
		r.client.Headers = map[string]string{"X-Redmine-API-Key": r.token}
	} else if r.username != "" && r.password != "" {
		r.client.BasicUser, r.client.BasicPass = r.username, r.password
	}

	reg := func(name, desc string, fn, params interface{}) {
		on, set := r.enabled[name]
		if r.all || !set || on {
			r.Toolkit.Register(name, desc, r, fn, params)
		}
	}
	reg("GetIssue", "Retrieve issue details from Redmine.", r.GetIssue, GetIssueParams{})
	reg("CreateIssue", "Create a new issue in Redmine. Returns the new issue's id and URL.", r.CreateIssue, CreateIssueParams{})
	reg("UpdateIssue", "Update an existing issue in Redmine. Only the provided fields are changed. Use this to move an issue to a new status, reassign it, or set its priority.", r.UpdateIssue, UpdateIssueParams{})
	reg("SearchIssues", "Search for issues, optionally filtering by subject, project, status, tracker and assignee.", r.SearchIssues, SearchIssuesParams{})
	reg("AddComment", "Add a comment to an issue.", r.AddComment, AddCommentParams{})
	reg("LogTime", "Log time spent on an issue.", r.LogTime, LogTimeParams{})
	reg("ListProjects", "List the projects available on the Redmine server.", r.ListProjects, NoParams{})
	reg("ListUsers", "List the active users on the Redmine server. Use this to resolve an assignee name to the numeric id required by create_issue and update_issue. This requires an admin token; with a non-admin token use list_project_members instead.", r.ListUsers, NoParams{})
	reg("ListProjectMembers", "List the users who are members of a project. An issue can only be assigned to a member of its project, so use this to find valid assignee ids.", r.ListProjectMembers, ProjectParams{})
	reg("ListVersions", "List the versions (sprints/milestones) of a project. Use this to resolve a version name to the numeric id required by create_issue and update_issue.", r.ListVersions, ProjectParams{})
	return r
}

func (r *RedmineTools) check() error {
	if r.serverURL == "" {
		return fmt.Errorf("Redmine server URL not provided (set REDMINE_SERVER_URL)")
	}
	return nil
}

func errJSON(err error) string { return restkit.ErrorJSON(err.Error()) }

func toInt(id string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return 0, fmt.Errorf("Invalid issue id: '%s'", id)
	}
	return n, nil
}

// fetchAll pages through a Redmine collection (limit<=0 means everything).
func (r *RedmineTools) fetchAll(path, key string, q url.Values, limit int) ([]map[string]interface{}, error) {
	if q == nil {
		q = url.Values{}
	}
	out := []map[string]interface{}{}
	for offset := 0; offset < 100000; {
		pageSize := 100
		if limit > 0 && limit-len(out) < pageSize {
			pageSize = limit - len(out)
		}
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))
		var resp map[string]interface{}
		if err := r.client.Into(http.MethodGet, path, q, nil, &resp); err != nil {
			return nil, err
		}
		items := restkit.Slice(resp, key)
		out = append(out, items...)
		offset += len(items)
		total, hasTotal := resp["total_count"].(float64)
		if len(items) == 0 || (limit > 0 && len(out) >= limit) || !hasTotal || offset >= int(total) {
			break
		}
	}
	return out, nil
}

func pyList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "'" + n + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// resolveName maps a resource name (case-insensitive) to its id, or returns
// Python's {"error": "Unknown <label> ..."} JSON.
func resolveName(name string, resources []map[string]interface{}, label string) (interface{}, string) {
	mapping := map[string]interface{}{}
	for _, res := range resources {
		mapping[strings.ToLower(restkit.Str(res, "name"))] = res["id"]
	}
	if id, ok := mapping[strings.ToLower(name)]; ok {
		return id, ""
	}
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return nil, restkit.ErrorJSON(fmt.Sprintf("Unknown %s '%s'. Available: %s", label, name, pyList(keys)))
}

func (r *RedmineTools) resolve(kind, name string) (interface{}, string, error) {
	var path, key, label string
	switch kind {
	case "tracker":
		path, key, label = "/trackers.json", "trackers", "tracker"
	case "priority":
		path, key, label = "/enumerations/issue_priorities.json", "issue_priorities", "priority"
	case "status":
		path, key, label = "/issue_statuses.json", "issue_statuses", "status"
	}
	var resp map[string]interface{}
	if err := r.client.Into(http.MethodGet, path, nil, nil, &resp); err != nil {
		return nil, "", err
	}
	id, errMsg := resolveName(name, restkit.Slice(resp, key), label)
	return id, errMsg, nil
}

func refName(m map[string]interface{}, key string) string {
	return restkit.Str(restkit.Map(m, key), "name")
}

// GetIssue retrieves issue details.
func (r *RedmineTools) GetIssue(p GetIssueParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	id, err := toInt(p.IssueID)
	if err != nil {
		return errJSON(err), nil
	}
	var q url.Values
	if p.IncludeComments {
		q = url.Values{"include": {"journals"}}
	}
	var resp struct {
		Issue map[string]interface{} `json:"issue"`
	}
	if err := r.client.Into(http.MethodGet, fmt.Sprintf("/issues/%d.json", id), q, nil, &resp); err != nil {
		return errJSON(err), nil
	}
	is := resp.Issue
	assignee := refName(is, "assigned_to")
	if assignee == "" {
		assignee = "Unassigned"
	}
	var parentID interface{}
	if parent := restkit.Map(is, "parent"); parent != nil {
		parentID = parent["id"]
	}
	details := map[string]interface{}{
		"id":              is["id"],
		"project":         refName(is, "project"),
		"tracker":         refName(is, "tracker"),
		"status":          refName(is, "status"),
		"priority":        refName(is, "priority"),
		"author":          refName(is, "author"),
		"assignee":        assignee,
		"subject":         is["subject"],
		"description":     restkit.Str(is, "description"),
		"done_ratio":      is["done_ratio"],
		"version":         refName(is, "fixed_version"),
		"parent_id":       parentID,
		"estimated_hours": is["estimated_hours"],
	}
	if p.IncludeComments {
		comments := []string{}
		for _, j := range restkit.Slice(is, "journals") {
			if n := restkit.Str(j, "notes"); n != "" {
				comments = append(comments, n)
			}
		}
		details["comments"] = comments
	}
	return restkit.CompactJSON(details), nil
}

// issueFields builds the shared optional fields of create/update.
func (r *RedmineTools) issueFields(fields map[string]interface{}, status, tracker, priority string) (string, error) {
	for _, kv := range [][3]string{{"status", status, "status_id"}, {"tracker", tracker, "tracker_id"}, {"priority", priority, "priority_id"}} {
		if kv[1] == "" {
			continue
		}
		id, errMsg, err := r.resolve(kv[0], kv[1])
		if err != nil {
			return "", err
		}
		if errMsg != "" {
			return errMsg, nil
		}
		fields[kv[2]] = id
	}
	return "", nil
}

// CreateIssue creates an issue and returns {"id", "url"}.
func (r *RedmineTools) CreateIssue(p CreateIssueParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{"project_id": p.ProjectID, "subject": p.Subject, "description": p.Description}
	if msg, err := r.issueFields(fields, "", p.Tracker, p.Priority); err != nil {
		return errJSON(err), nil
	} else if msg != "" {
		return msg, nil
	}
	setOptional(fields, p.AssignedToID, p.VersionID, p.ParentIssueID, p.EstimatedHours, p.StartDate, p.DueDate)
	var resp struct {
		Issue struct {
			ID int `json:"id"`
		} `json:"issue"`
	}
	if err := r.client.Into(http.MethodPost, "/issues.json", nil, map[string]interface{}{"issue": fields}, &resp); err != nil {
		return errJSON(err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"id": resp.Issue.ID, "url": fmt.Sprintf("%s/issues/%d", r.serverURL, resp.Issue.ID)}), nil
}

func setOptional(fields map[string]interface{}, assignedTo, version, parent int, hours float64, start, due string) {
	if assignedTo != 0 {
		fields["assigned_to_id"] = assignedTo
	}
	if version != 0 {
		fields["fixed_version_id"] = version
	}
	if parent != 0 {
		fields["parent_issue_id"] = parent
	}
	if hours != 0 {
		fields["estimated_hours"] = hours
	}
	if start != "" {
		fields["start_date"] = start
	}
	if due != "" {
		fields["due_date"] = due
	}
}

// UpdateIssue changes only the provided fields.
func (r *RedmineTools) UpdateIssue(p UpdateIssueParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if p.Subject != "" {
		fields["subject"] = p.Subject
	}
	if p.Description != "" {
		fields["description"] = p.Description
	}
	if msg, err := r.issueFields(fields, p.Status, p.Tracker, p.Priority); err != nil {
		return errJSON(err), nil
	} else if msg != "" {
		return msg, nil
	}
	if p.DoneRatio != 0 {
		fields["done_ratio"] = p.DoneRatio
	}
	setOptional(fields, p.AssignedToID, p.VersionID, p.ParentIssueID, p.EstimatedHours, p.StartDate, p.DueDate)
	if len(fields) == 0 {
		return restkit.ErrorJSON("No fields provided to update."), nil
	}
	id, err := toInt(p.IssueID)
	if err != nil {
		return errJSON(err), nil
	}
	if _, _, err := r.client.Raw(http.MethodPut, fmt.Sprintf("/issues/%d.json", id), nil, map[string]interface{}{"issue": fields}); err != nil {
		return errJSON(err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"status": "success", "issue_id": p.IssueID}), nil
}

// SearchIssues searches issues with optional filters.
func (r *RedmineTools) SearchIssues(p SearchIssuesParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	maxResults := p.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	q := url.Values{"status_id": {"*"}}
	if p.Pattern != "" {
		q.Set("subject", "~"+p.Pattern)
	}
	if p.ProjectID != "" {
		q.Set("project_id", p.ProjectID)
	}
	if p.AssignedToID != 0 {
		q.Set("assigned_to_id", strconv.Itoa(p.AssignedToID))
	}
	if p.Tracker != "" {
		id, msg, err := r.resolve("tracker", p.Tracker)
		if err != nil {
			return errJSON(err), nil
		}
		if msg != "" {
			return msg, nil
		}
		q.Set("tracker_id", fmt.Sprint(id))
	}
	if p.Status != "" {
		switch lowered := strings.ToLower(p.Status); lowered {
		case "open", "closed":
			q.Set("status_id", lowered)
		case "all", "*":
			q.Set("status_id", "*")
		default:
			id, msg, err := r.resolve("status", p.Status)
			if err != nil {
				return errJSON(err), nil
			}
			if msg != "" {
				return msg, nil
			}
			q.Set("status_id", fmt.Sprint(id))
		}
	}
	issues, err := r.fetchAll("/issues.json", "issues", q, maxResults)
	if err != nil {
		return errJSON(err), nil
	}
	results := []map[string]interface{}{}
	for _, is := range issues {
		assignee := refName(is, "assigned_to")
		if assignee == "" {
			assignee = "Unassigned"
		}
		results = append(results, map[string]interface{}{
			"id": is["id"], "subject": is["subject"], "tracker": refName(is, "tracker"),
			"status": refName(is, "status"), "priority": refName(is, "priority"), "assignee": assignee,
		})
	}
	return restkit.CompactJSON(results), nil
}

// AddComment adds a journal note to an issue.
func (r *RedmineTools) AddComment(p AddCommentParams) (interface{}, error) {
	if strings.TrimSpace(p.Comment) == "" {
		return restkit.ErrorJSON("comment cannot be empty"), nil
	}
	if err := r.check(); err != nil {
		return nil, err
	}
	id, err := toInt(p.IssueID)
	if err != nil {
		return errJSON(err), nil
	}
	body := map[string]interface{}{"issue": map[string]interface{}{"notes": p.Comment, "private_notes": p.PrivateNotes}}
	if _, _, err := r.client.Raw(http.MethodPut, fmt.Sprintf("/issues/%d.json", id), nil, body); err != nil {
		return errJSON(err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"status": "success", "issue_id": p.IssueID}), nil
}

// LogTime creates a time entry on an issue.
func (r *RedmineTools) LogTime(p LogTimeParams) (interface{}, error) {
	if p.Hours <= 0 {
		return restkit.ErrorJSON("hours must be greater than 0"), nil
	}
	if err := r.check(); err != nil {
		return nil, err
	}
	id, err := toInt(p.IssueID)
	if err != nil {
		return errJSON(err), nil
	}
	fields := map[string]interface{}{"issue_id": id, "hours": p.Hours}
	var resp map[string]interface{}
	if err := r.client.Into(http.MethodGet, "/enumerations/time_entry_activities.json", nil, nil, &resp); err != nil {
		return errJSON(err), nil
	}
	activities := restkit.Slice(resp, "time_entry_activities")
	if p.Activity != "" {
		aid, msg := resolveName(p.Activity, activities, "activity")
		if msg != "" {
			return msg, nil
		}
		fields["activity_id"] = aid
	} else {
		hasDefault := false
		names := []string{}
		for _, a := range activities {
			if d, _ := a["is_default"].(bool); d {
				hasDefault = true
			}
			names = append(names, strings.ToLower(restkit.Str(a, "name")))
		}
		if !hasDefault {
			sort.Strings(names)
			return restkit.ErrorJSON("activity is required (this Redmine instance defines no default activity). Available: " + pyList(names)), nil
		}
	}
	if p.Comment != "" {
		fields["comments"] = p.Comment
	}
	if p.SpentOn != "" {
		fields["spent_on"] = p.SpentOn
	}
	var created struct {
		TimeEntry struct {
			ID int `json:"id"`
		} `json:"time_entry"`
	}
	if err := r.client.Into(http.MethodPost, "/time_entries.json", nil, map[string]interface{}{"time_entry": fields}, &created); err != nil {
		return errJSON(err), nil
	}
	return restkit.CompactJSON(map[string]interface{}{"id": created.TimeEntry.ID, "issue_id": p.IssueID, "hours": p.Hours}), nil
}

// ListProjects lists projects (id, identifier, name).
func (r *RedmineTools) ListProjects(NoParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	items, err := r.fetchAll("/projects.json", "projects", nil, 0)
	if err != nil {
		return errJSON(err), nil
	}
	out := []map[string]interface{}{}
	for _, p := range items {
		out = append(out, map[string]interface{}{"id": p["id"], "identifier": p["identifier"], "name": p["name"]})
	}
	return restkit.CompactJSON(out), nil
}

// ListUsers lists active users (id, name, login). Requires an admin token.
func (r *RedmineTools) ListUsers(NoParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	items, err := r.fetchAll("/users.json", "users", nil, 0)
	if err != nil {
		return errJSON(err), nil
	}
	out := []map[string]interface{}{}
	for _, u := range items {
		name := strings.TrimSpace(restkit.Str(u, "firstname") + " " + restkit.Str(u, "lastname"))
		out = append(out, map[string]interface{}{"id": u["id"], "name": name, "login": u["login"]})
	}
	return restkit.CompactJSON(out), nil
}

// ListProjectMembers lists the user members of a project.
func (r *RedmineTools) ListProjectMembers(p ProjectParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	items, err := r.fetchAll("/projects/"+url.PathEscape(p.ProjectID)+"/memberships.json", "memberships", nil, 0)
	if err != nil {
		return errJSON(err), nil
	}
	out := []map[string]interface{}{}
	for _, m := range items {
		if u := restkit.Map(m, "user"); u != nil {
			out = append(out, map[string]interface{}{"id": u["id"], "name": restkit.Str(u, "name")})
		}
	}
	return restkit.CompactJSON(out), nil
}

// ListVersions lists the versions of a project.
func (r *RedmineTools) ListVersions(p ProjectParams) (interface{}, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	items, err := r.fetchAll("/projects/"+url.PathEscape(p.ProjectID)+"/versions.json", "versions", nil, 0)
	if err != nil {
		return errJSON(err), nil
	}
	out := []map[string]interface{}{}
	for _, v := range items {
		out = append(out, map[string]interface{}{"id": v["id"], "name": v["name"], "status": restkit.Str(v, "status")})
	}
	return restkit.CompactJSON(out), nil
}

// Execute implements toolkit.Tool.
func (r *RedmineTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return r.Toolkit.Execute(methodName, input)
}

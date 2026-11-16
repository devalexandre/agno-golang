package redmine

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newServer(t *testing.T, bodies map[string]*map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Redmine-API-Key") != "key" {
			t.Errorf("missing api key on %s", r.URL.Path)
		}
		if b, ok := bodies[r.Method+" "+r.URL.Path]; ok {
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, b)
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /issues/7.json":
			if r.URL.Query().Get("include") != "journals" {
				t.Errorf("missing include")
			}
			_, _ = w.Write([]byte(`{"issue":{"id":7,"subject":"Bug","project":{"id":1,"name":"Web"},"status":{"name":"New"},"journals":[{"notes":"hi"},{"notes":""}]}}`))
		case "GET /trackers.json":
			_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug"},{"id":2,"name":"Feature"}]}`))
		case "POST /issues.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"issue":{"id":42}}`))
		case "PUT /issues/7.json":
			w.WriteHeader(http.StatusNoContent)
		case "GET /issues.json":
			q := r.URL.Query()
			if q.Get("status_id") != "open" || q.Get("subject") != "~login" || q.Get("limit") != "5" {
				t.Errorf("bad search query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"issues":[{"id":3,"subject":"login fails","tracker":{"name":"Bug"}}],"total_count":1}`))
		}
	}))
}

func TestRedmine(t *testing.T) {
	var created, updated map[string]interface{}
	srv := newServer(t, map[string]*map[string]interface{}{"POST /issues.json": &created, "PUT /issues/7.json": &updated})
	defer srv.Close()
	r := New(WithServerURL(srv.URL), WithToken("key"))

	out, _ := r.GetIssue(GetIssueParams{IssueID: "7", IncludeComments: true})
	var issue map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &issue)
	if issue["project"] != "Web" || issue["assignee"] != "Unassigned" || len(issue["comments"].([]interface{})) != 1 {
		t.Fatalf("bad issue: %v", out)
	}

	out, _ = r.CreateIssue(CreateIssueParams{ProjectID: "web", Subject: "S", Description: "D", Tracker: "feature", VersionID: 3})
	if out != `{"id":42,"url":"`+srv.URL+`/issues/42"}` {
		t.Fatalf("bad create: %v", out)
	}
	fields := created["issue"].(map[string]interface{})
	if fields["tracker_id"].(float64) != 2 || fields["fixed_version_id"].(float64) != 3 || fields["project_id"] != "web" {
		t.Fatalf("bad create body: %v", created)
	}

	out, _ = r.CreateIssue(CreateIssueParams{ProjectID: "web", Subject: "S", Description: "D", Tracker: "Task"})
	if out != `{"error":"Unknown tracker 'Task'. Available: ['bug', 'feature']"}` {
		t.Fatalf("bad resolve error: %v", out)
	}

	out, _ = r.AddComment(AddCommentParams{IssueID: "7", Comment: "done"})
	if out != `{"issue_id":"7","status":"success"}` || updated["issue"].(map[string]interface{})["notes"] != "done" {
		t.Fatalf("bad comment: %v %v", out, updated)
	}

	out, _ = r.SearchIssues(SearchIssuesParams{Pattern: "login", Status: "Open", MaxResults: 5})
	if out != `[{"assignee":"Unassigned","id":3,"priority":"","status":"","subject":"login fails","tracker":"Bug"}]` {
		t.Fatalf("bad search: %v", out)
	}

	if out, _ := r.UpdateIssue(UpdateIssueParams{IssueID: "7"}); out != `{"error":"No fields provided to update."}` {
		t.Fatalf("bad empty update: %v", out)
	}
	if len(r.GetMethods()) != 10 || len(New(WithServerURL("x"), WithEnableLogTime(false)).GetMethods()) != 9 {
		t.Fatal("bad method registration")
	}
}

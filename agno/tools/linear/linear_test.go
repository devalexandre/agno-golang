package linear

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLinear(t *testing.T) {
	var gotAuth, gotMethod string
	var gotBody struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		switch {
		case strings.Contains(gotBody.Query, "issueCreate"):
			_, _ = w.Write([]byte(`{"data":{"issueCreate":{"success":true,"issue":{"id":"i1","title":"T","url":"u"}}}}`))
		case strings.Contains(gotBody.Query, "workflowState"):
			_, _ = w.Write([]byte(`{"errors":[{"message":"not found"}]}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"teams":{"nodes":[{"id":"t1","name":"Eng"}]}}}`))
		}
	}))
	defer srv.Close()

	l := New(WithAPIKey("lin_key"), WithBaseURL(srv.URL))
	out, err := l.CreateIssue(CreateIssueParams{Title: "T", Description: "D", TeamID: "t1", AssigneeID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "lin_key" || gotMethod != http.MethodPost || gotBody.Variables["teamId"] != "t1" || gotBody.Variables["assigneeId"] != "a1" {
		t.Fatalf("unexpected request: auth=%s method=%s vars=%v", gotAuth, gotMethod, gotBody.Variables)
	}
	if _, ok := gotBody.Variables["projectId"]; ok {
		t.Fatal("projectId should be omitted")
	}
	if out != `{"id":"i1","title":"T","url":"u"}` {
		t.Fatalf("unexpected output: %v", out)
	}

	out, err = l.GetTeamsDetails(NoParams{})
	if err != nil || out != `[{"id":"t1","name":"Eng"}]` {
		t.Fatalf("unexpected teams: %v %v", out, err)
	}

	if _, err := l.GetWorkflowIssues(WorkflowParams{WorkflowID: "w"}); err == nil || !strings.Contains(err.Error(), "GraphQL Error") {
		t.Fatalf("expected GraphQL error, got %v", err)
	}
	if len(l.GetMethods()) != 8 {
		t.Fatalf("expected 8 methods, got %d", len(l.GetMethods()))
	}
}

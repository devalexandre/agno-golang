package gitlab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListMergeRequestsAndIssues(t *testing.T) {
	var gotURI, gotToken, gotState, gotPerPage string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		gotState = r.URL.Query().Get("state")
		gotPerPage = r.URL.Query().Get("per_page")
		switch r.URL.EscapedPath() {
		case "/api/v4/projects/group%2Fproj/merge_requests":
			_, _ = w.Write([]byte(`[{"id":1,"iid":7,"title":"Fix","state":"opened","author":{"username":"alice"},"source_branch":"fix"}]`))
		case "/api/v4/projects/group%2Fproj/issues":
			_, _ = w.Write([]byte(`[{"id":2,"iid":3,"title":"Bug","labels":["bug"],"author":{"username":"bob"},"assignees":[{"username":"carol"}]}]`))
		case "/api/v4/projects/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.EscapedPath())
		}
	}))
	defer srv.Close()

	g := New(WithAccessToken("tok"), WithBaseURL(srv.URL+"/"))
	out, err := g.ListMergeRequests(ListMergeRequestsParams{ProjectIDOrPath: "group/proj", PerPage: 500})
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "tok" || gotState != "opened" || gotPerPage != "100" {
		t.Fatalf("unexpected request: uri=%s token=%s state=%s per_page=%s", gotURI, gotToken, gotState, gotPerPage)
	}
	var res struct {
		Data []map[string]interface{} `json:"data"`
		Meta map[string]int           `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data) != 1 || res.Data[0]["author"] != "alice" || res.Meta["returned_items"] != 1 || res.Meta["current_page"] != 1 {
		t.Fatalf("unexpected output: %s", out)
	}

	out, _ = g.ListIssues(ListIssuesParams{ProjectIDOrPath: "group/proj"})
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if a, _ := res.Data[0]["assignees"].([]interface{}); len(a) != 1 || a[0] != "carol" {
		t.Fatalf("unexpected issue output: %s", out)
	}

	out, _ = g.GetProject(GetProjectParams{ProjectIDOrPath: "missing"})
	if out != `{"error":"404: 404 Project Not Found"}` {
		t.Fatalf("unexpected error output: %v", out)
	}
	if len(g.GetMethods()) != 5 {
		t.Fatalf("expected 5 methods, got %d", len(g.GetMethods()))
	}
}

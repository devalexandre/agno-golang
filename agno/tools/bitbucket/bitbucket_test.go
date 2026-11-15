package bitbucket

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBitbucket(t *testing.T) {
	var commitCalls int
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "u" || pass != "tok" {
			t.Errorf("bad auth: %v %s %s", ok, user, pass)
		}
		switch r.URL.Path {
		case "/2.0/repositories/ws/repo/commits":
			commitCalls++
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("page") == "2" {
				_, _ = w.Write([]byte(`{"values":[{"hash":"c"},{"hash":"d"}]}`))
				return
			}
			if r.URL.Query().Get("pagelen") != "3" {
				t.Errorf("pagelen=%s", r.URL.Query().Get("pagelen"))
			}
			_, _ = w.Write([]byte(`{"values":[{"hash":"a"},{"hash":"b"}],"next":"https://api.bitbucket.org/2.0/repositories/ws/repo/commits?pagelen=3&page=2"}`))
		case "/2.0/repositories/ws/repo/pullrequests/5/diff":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("diff --git a/x b/x"))
		case "/2.0/repositories/ws/repo/pullrequests":
			if r.URL.Query().Get("state") != "OPEN" {
				t.Errorf("state=%s", r.URL.Query().Get("state"))
			}
			_, _ = w.Write([]byte(`{"values":[]}`))
		case "/2.0/repositories/ws/my-new-repo":
			if r.Method != http.MethodPost {
				t.Errorf("method=%s", r.Method)
			}
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &gotBody)
			_, _ = w.Write([]byte(`{"slug":"my-new-repo"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	b := New(WithBaseURL(srv.URL+"/2.0"), WithUsername("u"), WithToken("tok"), WithPassword("ignored"), WithWorkspace("ws"), WithRepoSlug("repo"))
	out, err := b.ListRepositoryCommits(CountParams{Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Values []map[string]string `json:"values"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &page); err != nil {
		t.Fatal(err)
	}
	if commitCalls != 2 || len(page.Values) != 3 || page.Values[2]["hash"] != "c" {
		t.Fatalf("unexpected commits (%d calls): %s", commitCalls, out)
	}

	diff, _ := b.GetPullRequestChanges(PullRequestParams{PullRequestID: 5})
	if diff != "diff --git a/x b/x" {
		t.Fatalf("unexpected diff: %v", diff)
	}
	if _, err := b.ListAllPullRequests(ListPullRequestsParams{State: "bogus"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CreateRepository(CreateRepositoryParams{Name: "My New Repo", Project: "PRJ", IsPrivate: true}); err != nil {
		t.Fatal(err)
	}
	if gotBody["scm"] != "git" || gotBody["is_private"] != true || gotBody["project"].(map[string]interface{})["key"] != "PRJ" {
		t.Fatalf("unexpected create body: %v", gotBody)
	}

	missing := New(WithBaseURL(srv.URL), WithUsername("u"), WithToken("t"))
	if out, _ := missing.GetRepositoryDetails(NoParams{}); out != `{"error":"Workspace is required"}` {
		t.Fatalf("unexpected: %v", out)
	}
}

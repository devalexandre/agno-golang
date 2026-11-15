package apify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApifyActorTool(t *testing.T) {
	var auth, runQuery string
	var runBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.Method + " " + r.URL.EscapedPath() {
		case "GET /acts/apify~rag-web-browser":
			_, _ = w.Write([]byte(`{"data":{"id":"abc123"}}`))
		case "GET /acts/abc123/builds/default":
			_, _ = w.Write([]byte(`{"data":{"actorDefinition":{"description":"Browse the web","input":{
				"properties":{"query":{"type":"string","description":"Search query"},
				"maxResults":{"type":"integer","default":3},
				"urls":{"type":"array","prefill":["https://a"]}},
				"required":["query"]}}}}`))
		case "POST /acts/apify~rag-web-browser/run-sync-get-dataset-items":
			runQuery = r.URL.RawQuery
			_ = json.NewDecoder(r.Body).Decode(&runBody)
			_, _ = w.Write([]byte(`[{"url":"https://go.dev","markdown":"# Go"}]`))
		default:
			http.Error(w, "unexpected "+r.URL.EscapedPath(), http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a := New(WithAPIToken("tok"), WithBaseURL(srv.URL), WithActors("apify/rag-web-browser"))
	name := "ApifyTools_apify_actor_apify_rag-web-browser"
	m, ok := a.GetMethods()[name]
	if !ok {
		t.Fatalf("actor tool not registered: %v", a.GetMethods())
	}
	props := m.Schema["properties"].(map[string]interface{})
	if m.Schema["required"].([]string)[0] != "query" || props["urls"].(map[string]interface{})["items"].(map[string]interface{})["type"] != "string" {
		t.Fatalf("unexpected schema: %v", m.Schema)
	}
	if !strings.Contains(m.Description, "Browse the web") || !strings.Contains(m.Description, "query (string): (required)") {
		t.Fatalf("unexpected description: %s", m.Description)
	}

	out, err := a.Execute(name, json.RawMessage(`{"query":"golang","maxResults":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok" || runQuery != "clean=true" || runBody["query"] != "golang" || runBody["maxResults"] != float64(2) {
		t.Fatalf("unexpected run request: auth=%s q=%s body=%v", auth, runQuery, runBody)
	}
	if !strings.Contains(out.(string), "https://go.dev") {
		t.Fatalf("unexpected output: %v", out)
	}
}

func TestApifyNoToken(t *testing.T) {
	t.Setenv("APIFY_API_TOKEN", "")
	a := New(WithActors("apify/rag-web-browser"))
	if len(a.GetMethods()) != 0 {
		t.Fatal("expected no tools without a token")
	}
	if err := a.RegisterActor("x"); err == nil {
		t.Fatal("expected error without token")
	}
}

package agentql

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentQL(t *testing.T) {
	var key, path string
	var bodies []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("X-API-Key")
		path = r.Method + " " + r.URL.Path
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		if strings.Contains(b["query"].(string), "text_content") {
			_, _ = w.Write([]byte(`{"data":{"text_content":["Hello","  ","World","Hello"]},"metadata":{"request_id":"x"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"products":[{"name":"A"}]}}`))
	}))
	defer srv.Close()

	a := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithAgentQLQuery("{ products[] { name } }"))
	if len(a.GetMethods()) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(a.GetMethods()))
	}
	out, _ := a.ScrapeWebsite(URLParams{URL: "https://go.dev"})
	if out != "Hello World" || key != "k" || path != "POST /query-data" || bodies[0]["url"] != "https://go.dev" {
		t.Fatalf("unexpected scrape: %v key=%s path=%s", out, key, path)
	}
	out, _ = a.CustomScrapeWebsite(URLParams{URL: "https://shop"})
	if out != `{"products":[{"name":"A"}]}` || bodies[1]["query"] != "{ products[] { name } }" {
		t.Fatalf("unexpected custom scrape: %v", out)
	}
	if out, _ := a.ScrapeWebsite(URLParams{}); out != "No URL provided" {
		t.Fatalf("unexpected: %v", out)
	}
}

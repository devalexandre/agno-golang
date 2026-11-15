package seltz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSeltz(t *testing.T) {
	var key, path string
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("x-api-key")
		path = r.Method + " " + r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"documents":[{"url":"https://go.dev","content":"Go <3"},{"url":null,"content":null}]}`))
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithMaxResults(3))
	out, err := s.SearchSeltz(SearchParams{Query: "golang", Scope: "news", IncludeDomains: []string{"go.dev"}})
	if err != nil {
		t.Fatal(err)
	}
	if key != "k" || path != "POST /v1/search" || body["max_results"] != float64(3) || body["scope"] != "news" || body["include_domains"] == nil {
		t.Fatalf("unexpected request: key=%s path=%s body=%v", key, path, body)
	}
	want := "[\n    {\n        \"content\": \"Go <3\",\n        \"url\": \"https://go.dev\"\n    }\n]"
	if out != want {
		t.Fatalf("unexpected output: %q", out)
	}
	if out, _ := s.SearchSeltz(SearchParams{Query: "x", MaxResults: -1}); out != "Error: max_results must be greater than 0." {
		t.Fatalf("unexpected: %v", out)
	}
}

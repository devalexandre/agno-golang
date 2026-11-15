package sofya

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSofya(t *testing.T) {
	var auth string
	bodies := map[string]map[string]interface{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies[r.URL.Path] = b
		switch r.URL.Path {
		case "/v1/search":
			_, _ = w.Write([]byte(`{"answer":"Go is a language","results":[{"title":"Go","url":"https://go.dev","content":"body"}]}`))
		case "/v1/fetch":
			_, _ = w.Write([]byte(`{"results":[{"url":"https://a","content":"A"},{"url":"https://b","success":false,"error":"blocked"}]}`))
		case "/v1/research":
			_, _ = w.Write([]byte(`{"report":"R","sources":[{"title":"S","url":"https://s"},"plain"]}`))
		}
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL+"/"), WithAll())
	if len(s.GetMethods()) != 3 {
		t.Fatalf("expected 3 methods")
	}
	out, _ := s.SearchWeb(SearchWebParams{Query: "go", MaxResults: 50})
	want := "# go\n\n### Summary\nGo is a language\n\n### [Go](https://go.dev)\nbody\n\n"
	if out != want || auth != "Bearer k" || bodies["/v1/search"]["max_results"] != float64(20) || bodies["/v1/search"]["search_depth"] != "basic" {
		t.Fatalf("unexpected search: %q body=%v", out, bodies["/v1/search"])
	}
	out, _ = s.ExtractURLContent(ExtractParams{URLs: "https://a, https://b"})
	if !strings.Contains(out.(string), "## https://a\n\nA") || !strings.Contains(out.(string), "**Extraction failed**: blocked") {
		t.Fatalf("unexpected extract: %q", out)
	}
	out, _ = s.Research(ResearchParams{Query: "q"})
	if out != "# q\n\nR\n\n## Sources\n- [S](https://s)\n- plain\n" {
		t.Fatalf("unexpected research: %q", out)
	}
	if out, _ := s.SearchWeb(SearchWebParams{Query: "  "}); !strings.Contains(out.(string), "Please provide a query") {
		t.Fatalf("unexpected: %v", out)
	}
}

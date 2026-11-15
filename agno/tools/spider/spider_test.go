package spider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSpider(t *testing.T) {
	var paths []string
	var bodies []map[string]interface{}
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		paths = append(paths, r.Method+" "+r.URL.Path)
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		_, _ = w.Write([]byte(`[{"url":"https://go.dev","content":"# Go"}]`))
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithMaxResults(3))
	out, err := s.SearchWeb(SearchWebParams{Query: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.(string), "https://go.dev") {
		t.Fatalf("unexpected output: %v", out)
	}
	if _, err := s.Crawl(CrawlParams{URL: "https://go.dev"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scrape(ScrapeParams{URL: "https://go.dev"}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer k" || paths[0] != "POST /search" || paths[1] != "POST /crawl" {
		t.Fatalf("unexpected requests: %v auth=%s", paths, auth)
	}
	if bodies[0]["search"] != "golang" || bodies[0]["num"] != float64(3) || bodies[0]["fetch_page_content"] != false {
		t.Fatalf("unexpected search body: %v", bodies[0])
	}
	if bodies[1]["limit"] != float64(10) || bodies[1]["return_format"] != "markdown" || bodies[2]["limit"] != float64(1) {
		t.Fatalf("unexpected crawl/scrape bodies: %v", bodies[1:])
	}
	if len(s.GetMethods()) != 3 {
		t.Fatalf("expected 3 methods, got %d", len(s.GetMethods()))
	}
}

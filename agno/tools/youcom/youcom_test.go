package youcom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestYouSearch(t *testing.T) {
	var gotPath, gotKey string
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey, gotQuery = r.URL.Path, r.Header.Get("X-API-Key"), r.URL.Query()
		_, _ = w.Write([]byte(`{"results":{"web":[{"url":"https://go.dev","title":"Go","description":"desc","snippets":["a","b"],"page_age":"2024"}],"news":[{"url":"https://n.com","title":"N","contents":{"markdown":"0123456789"}}]}}`))
	}))
	defer srv.Close()

	y := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithLivecrawl("web"), WithLivecrawlFormats("markdown,html"),
		WithTextLengthLimit(5), WithIncludeDomains("go.dev", "golang.org"))
	out, err := y.YouSearch(YouSearchParams{Query: "golang", NumResults: 3})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/search" || gotKey != "k" || gotQuery["query"][0] != "golang" || gotQuery["count"][0] != "3" ||
		gotQuery["livecrawl"][0] != "web" || len(gotQuery["livecrawl_formats"]) != 2 || gotQuery["crawl_timeout"][0] != "10" ||
		gotQuery["include_domains"][0] != "go.dev,golang.org" {
		t.Fatalf("unexpected request: %s %s %v", gotPath, gotKey, gotQuery)
	}
	var res []map[string]string
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0]["snippet"] != "desc" || res[0]["text"] != "a\nb" || res[0]["published_date"] != "2024" || res[1]["text"] != "01234" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestYouSearchMarkdownAndConfigErr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":[{"url":"https://go.dev","title":"Go"}]}`))
	}))
	defer srv.Close()
	out, err := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithFormat("markdown")).YouSearch(YouSearchParams{Query: "go"})
	if err != nil || !strings.Contains(out.(string), "## [Go](https://go.dev)") {
		t.Fatalf("unexpected: %v %v", out, err)
	}
	if _, err := New(WithAPIKey("k"), WithCrawlTimeout(100)).YouSearch(YouSearchParams{Query: "go"}); err == nil {
		t.Fatal("expected config error")
	}
}

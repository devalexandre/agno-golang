package scrapegraph

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrapeGraph(t *testing.T) {
	var key string
	var extractBody, crawlBody map[string]interface{}
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("SGAI-APIKEY")
		switch r.Method + " " + r.URL.Path {
		case "POST /extract":
			_ = json.NewDecoder(r.Body).Decode(&extractBody)
			_, _ = w.Write([]byte(`{"raw":null,"json":{"title":"Go"}}`))
		case "POST /scrape":
			_, _ = w.Write([]byte(`{"results":{"markdown":{"data":"# Go"}}}`))
		case "POST /crawl":
			_ = json.NewDecoder(r.Body).Decode(&crawlBody)
			_, _ = w.Write([]byte(`{"id":"c1","status":"running","total":0,"finished":0,"pages":[]}`))
		case "GET /crawl/c1":
			polls++
			_, _ = w.Write([]byte(`{"id":"c1","status":"completed","total":1,"finished":1,"pages":[]}`))
		case "POST /search":
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"detail":"no credits"}`))
		}
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithAll(), WithRenderHeavyJS(true), WithCrawlPollInterval(time.Millisecond))
	if len(s.GetMethods()) != 5 {
		t.Fatalf("expected 5 methods, got %d", len(s.GetMethods()))
	}
	out, _ := s.Smartscraper(SmartscraperParams{URL: "https://go.dev", Prompt: "title"})
	if out != `{"title":"Go"}` || key != "k" || extractBody["fetchConfig"].(map[string]interface{})["mode"] != "js" {
		t.Fatalf("unexpected smartscraper: %v body=%v", out, extractBody)
	}
	if out, _ := s.Markdownify(URLParams{URL: "https://go.dev"}); out != "# Go" {
		t.Fatalf("unexpected markdown: %v", out)
	}
	out, _ = s.Crawl(CrawlParams{URL: "https://go.dev", Prompt: "p", Schema: map[string]interface{}{"type": "object"}})
	if polls != 1 || !strings.Contains(out.(string), `"status":"completed"`) || crawlBody["maxDepth"] != float64(2) {
		t.Fatalf("unexpected crawl: %v polls=%d body=%v", out, polls, crawlBody)
	}
	out, _ = s.Searchscraper(SearchscraperParams{Query: "go"})
	if !strings.HasPrefix(out.(string), "Error searching: Insufficient credits") || !strings.Contains(out.(string), "no credits") {
		t.Fatalf("unexpected search error: %v", out)
	}
}

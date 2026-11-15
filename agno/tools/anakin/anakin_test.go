package anakin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScrapeUrlPollsJob(t *testing.T) {
	var body map[string]interface{}
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("bad auth %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/url-scraper":
			_ = json.NewDecoder(r.Body).Decode(&body)
			_, _ = w.Write([]byte(`{"jobId":"j1","status":"pending"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/url-scraper/j1":
			polls++
			if polls < 2 {
				_, _ = w.Write([]byte(`{"status":"processing"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"completed","markdown":"# Hi"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	a := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithPollInterval(time.Millisecond))
	out, err := a.ScrapeUrl(ScrapeURLParams{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if body["url"] != "https://example.com" || body["country"] != "us" || body["generateJson"] != false {
		t.Fatalf("unexpected body %v", body)
	}
	if !strings.Contains(out.(string), `"markdown":"# Hi"`) || polls != 2 {
		t.Fatalf("unexpected output %v polls=%d", out, polls)
	}
	if len(a.GetMethods()) != 1 {
		t.Fatalf("expected only ScrapeUrl registered, got %d", len(a.GetMethods()))
	}
}

func TestSearchWebAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crawl" {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte("no credits"))
			return
		}
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		if r.URL.Path != "/search" || b["prompt"] != "go" || b["limit"] != float64(5) {
			t.Errorf("unexpected search request %s %v", r.URL.Path, b)
		}
		_, _ = w.Write([]byte(`{"results":[{"url":"https://go.dev"}]}`))
	}))
	defer srv.Close()

	a := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithAll())
	if len(a.GetMethods()) != 4 {
		t.Fatalf("expected 4 methods, got %d", len(a.GetMethods()))
	}
	out, _ := a.SearchWeb(SearchWebParams{Query: "go"})
	if !strings.Contains(out.(string), "go.dev") {
		t.Fatalf("unexpected output %v", out)
	}
	out, _ = a.CrawlWebsite(CrawlWebsiteParams{URL: "https://x.com"})
	if out != "Error crawling https://x.com: 402 no credits" {
		t.Fatalf("unexpected error output %v", out)
	}
	t.Setenv("ANAKIN_API_KEY", "")
	if out, _ := New().ScrapeUrl(ScrapeURLParams{URL: "u"}); out != "Error: ANAKIN_API_KEY not set" {
		t.Fatalf("expected missing key error, got %v", out)
	}
}

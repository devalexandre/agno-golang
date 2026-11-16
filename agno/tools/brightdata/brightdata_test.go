package brightdata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBrightData(t *testing.T) {
	t.Setenv("BRIGHT_DATA_SERP_ZONE", "")
	t.Setenv("BRIGHT_DATA_WEB_UNLOCKER_ZONE", "")
	var auth, triggerQuery string
	var reqBodies []map[string]interface{}
	var triggerBody []map[string]interface{}
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.Method + " " + r.URL.Path {
		case "POST /request":
			var b map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&b)
			reqBodies = append(reqBodies, b)
			if b["data_format"] == "screenshot" {
				_, _ = w.Write([]byte("\x89PNG"))
				return
			}
			_, _ = w.Write([]byte("# Markdown"))
		case "POST /datasets/v3/trigger":
			triggerQuery = r.URL.RawQuery
			_ = json.NewDecoder(r.Body).Decode(&triggerBody)
			_, _ = w.Write([]byte(`{"snapshot_id":"s1"}`))
		case "GET /datasets/v3/snapshot/s1":
			polls++
			if polls == 1 {
				_, _ = w.Write([]byte(`{"status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`[{"title":"Product"}]`))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	b := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithScreenshotDir(dir), WithPollInterval(time.Millisecond))
	if len(b.GetMethods()) != 4 {
		t.Fatalf("expected 4 methods, got %d", len(b.GetMethods()))
	}
	out, _ := b.ScrapeAsMarkdown(URLParams{URL: "https://go.dev"})
	if out != "# Markdown" || auth != "Bearer k" || reqBodies[0]["zone"] != "web_unlocker1" {
		t.Fatalf("unexpected scrape: %v %v", out, reqBodies)
	}
	_, _ = b.SearchEngine(SearchEngineParams{Query: "go lang & more", Language: "en"})
	if reqBodies[1]["url"] != "https://www.google.com/search?q=go%20lang%20%26%20more&hl=en&num=10" || reqBodies[1]["zone"] != "serp_api" {
		t.Fatalf("unexpected search body: %v", reqBodies[1])
	}
	out, _ = b.GetScreenshot(URLParams{URL: "https://go.dev"})
	entries, _ := os.ReadDir(dir)
	if !strings.Contains(out.(string), "Screenshot captured") || len(entries) != 1 {
		t.Fatalf("unexpected screenshot: %v", out)
	}
	out, _ = b.WebDataFeed(WebDataFeedParams{SourceType: "amazon_product", URL: "https://amazon.com/dp/X"})
	if out != `[{"title":"Product"}]` || polls != 2 || !strings.Contains(triggerQuery, "dataset_id=gd_l7q7dkf244hwjntr0") || triggerBody[0]["url"] != "https://amazon.com/dp/X" {
		t.Fatalf("unexpected feed: %v polls=%d q=%s", out, polls, triggerQuery)
	}
	if out, _ := b.WebDataFeed(WebDataFeedParams{SourceType: "nope", URL: "u"}); !strings.HasPrefix(out.(string), "Invalid source_type: nope") {
		t.Fatalf("unexpected validation: %v", out)
	}
}

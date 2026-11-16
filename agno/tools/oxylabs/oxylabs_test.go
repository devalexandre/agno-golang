package oxylabs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOxylabs(t *testing.T) {
	var user, pass string
	var bodies []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		switch b["source"] {
		case "google_search":
			_, _ = w.Write([]byte(`{"results":[{"content":{"results":{"organic":[{"title":" Go ","url":"https://go.dev","desc":"lang","pos":1}]}}}]}`))
		case "amazon_product":
			_, _ = w.Write([]byte(`{"results":[{"content":{"title":"Kindle","price":99.5,"images":["a","b","c","d"]}}]}`))
		case "universal":
			_, _ = w.Write([]byte(`{"results":[{"content":"# Page","status_code":200}]}`))
		}
	}))
	defer srv.Close()

	o := New(WithUsername("u"), WithPassword("p"), WithBaseURL(srv.URL))
	if len(o.GetMethods()) != 4 {
		t.Fatalf("expected 4 methods")
	}
	out, _ := o.SearchGoogle(SearchGoogleParams{Query: " golang "})
	var g struct {
		Results []map[string]interface{} `json:"results"`
	}
	_ = json.Unmarshal([]byte(out.(string)), &g)
	if user != "u" || pass != "p" || bodies[0]["domain"] != "com" || bodies[0]["parse"] != true || bodies[0]["query"] != "golang" {
		t.Fatalf("unexpected google request: %v", bodies[0])
	}
	if len(g.Results) != 1 || g.Results[0]["title"] != "Go" || g.Results[0]["description"] != "lang" {
		t.Fatalf("unexpected google output: %v", out)
	}

	out, _ = o.GetAmazonProduct(AmazonProductParams{ASIN: "b07fz8s74r"})
	if bodies[1]["query"] != "B07FZ8S74R" || !strings.Contains(out.(string), `"found": true`) || strings.Contains(out.(string), `"d"`) {
		t.Fatalf("unexpected amazon product: %v", out)
	}
	if out, _ := o.GetAmazonProduct(AmazonProductParams{ASIN: "short"}); !strings.Contains(out.(string), "Invalid ASIN format") {
		t.Fatalf("expected ASIN validation, got %v", out)
	}

	out, _ = o.ScrapeWebsite(ScrapeWebsiteParams{URL: "https://go.dev", RenderJavascript: true, Markdown: true})
	if bodies[2]["markdown"] != true || bodies[2]["render"] != "html" || bodies[2]["parse"] != nil || !strings.Contains(out.(string), "# Page") {
		t.Fatalf("unexpected scrape: %v body=%v", out, bodies[2])
	}
	if out, _ := o.ScrapeWebsite(ScrapeWebsiteParams{URL: "ftp://x"}); !strings.Contains(out.(string), "Must start with http") {
		t.Fatalf("expected URL validation, got %v", out)
	}
}

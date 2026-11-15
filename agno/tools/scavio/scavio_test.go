package scavio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScavio(t *testing.T) {
	var auth string
	bodies := map[string]map[string]interface{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies[r.Method+" "+r.URL.Path] = b
		if r.URL.Path == "/api/v1/reddit/post" {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"error":"no credits"}`))
			return
		}
		_, _ = w.Write([]byte(`{"organic_results":[{"title":"Go","link":"https://go.dev"}]}`))
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if len(s.GetMethods()) != 9 {
		t.Fatalf("expected 9 methods, got %d", len(s.GetMethods()))
	}
	out, _ := s.GoogleSearch(GoogleSearchParams{Query: "golang", GL: "us"})
	g := bodies["POST /api/v2/google"]
	if auth != "Bearer k" || g["query"] != "golang" || g["gl"] != "us" || len(g) != 2 || !strings.Contains(out.(string), "https://go.dev") {
		t.Fatalf("unexpected google call: %v body=%v", out, g)
	}
	_, _ = s.AmazonProduct(AmazonProductParams{ASIN: "B09XS7JWHH"})
	if bodies["POST /api/v1/amazon/product"]["query"] != "B09XS7JWHH" {
		t.Fatalf("asin not renamed: %v", bodies["POST /api/v1/amazon/product"])
	}
	_, _ = s.YoutubeSearch(YouTubeSearchParams{Query: "go", HD: true})
	if y := bodies["POST /api/v1/youtube/search"]; y["search"] != "go" || y["hd"] != true {
		t.Fatalf("unexpected youtube body: %v", y)
	}
	out, _ = s.RedditPost(RedditPostParams{URL: "https://reddit.com/r/golang/x"})
	if !strings.HasPrefix(out.(string), `{"error":`) || !strings.Contains(out.(string), "402") {
		t.Fatalf("expected error JSON, got %v", out)
	}
}

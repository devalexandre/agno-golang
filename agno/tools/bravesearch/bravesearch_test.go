package bravesearch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBraveSearch(t *testing.T) {
	var gotToken, gotQuery, gotCount, gotFilter, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Subscription-Token")
		gotQuery = r.URL.Query().Get("q")
		gotCount = r.URL.Query().Get("count")
		gotFilter = r.URL.Query().Get("result_filter")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Go","url":"https://go.dev","description":"The Go language"}]}}`))
	}))
	defer srv.Close()

	b := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithFixedMaxResults(3))
	out, err := b.BraveSearch(SearchParams{Query: "golang", MaxResults: 10})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/web/search" || gotToken != "k" || gotQuery != "golang" || gotCount != "3" || gotFilter != "web" {
		t.Fatalf("unexpected request: path=%s token=%s q=%s count=%s filter=%s", gotPath, gotToken, gotQuery, gotCount, gotFilter)
	}
	var res struct {
		WebResults []map[string]string `json:"web_results"`
		Query      string              `json:"query"`
		Total      int                 `json:"total_results"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.WebResults[0]["url"] != "https://go.dev" || res.Query != "golang" {
		t.Fatalf("unexpected output: %s", out)
	}
	if _, ok := b.GetMethods()["BraveSearch"]; !ok {
		// method keys may be prefixed by toolkit name; just ensure something is registered
		if len(b.GetMethods()) != 1 {
			t.Fatalf("expected 1 method, got %v", b.GetMethods())
		}
	}
}

package perplexity

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPerplexitySearch(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotIntegration string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth, gotIntegration = r.Header.Get("Authorization"), r.Header.Get("X-Pplx-Integration")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"results":[{"url":"https://go.dev","title":"Go","snippet":"s","date":"2024-01-01"},{"url":"https://x.com"}]}`))
	}))
	defer srv.Close()

	p := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithSearchRecencyFilter("week"), WithSearchDomainFilter("go.dev"))
	out, err := p.Search(SearchParams{Query: "golang", MaxResults: 3})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/search" || gotAuth != "Bearer k" || gotIntegration != "agno" {
		t.Fatalf("unexpected request: %s %s %s %s", gotMethod, gotPath, gotAuth, gotIntegration)
	}
	if gotBody["query"] != "golang" || gotBody["max_results"].(float64) != 3 || gotBody["max_tokens_per_page"].(float64) != 2048 ||
		gotBody["search_recency_filter"] != "week" || len(gotBody["search_domain_filter"].([]interface{})) != 1 {
		t.Fatalf("unexpected body: %v", gotBody)
	}
	var res []map[string]string
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0]["title"] != "Go" || res[1]["url"] != "https://x.com" || len(res[1]) != 1 {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestPerplexityError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	out, err := New(WithAPIKey("k"), WithBaseURL(srv.URL)).Search(SearchParams{Query: "x"})
	if err != nil || !strings.HasPrefix(out.(string), `{"error":`) {
		t.Fatalf("unexpected: %v %v", out, err)
	}
}

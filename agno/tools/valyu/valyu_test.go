package valyu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchAcademicSources(t *testing.T) {
	var gotMethod, gotPath, gotKey string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotKey = r.Method, r.URL.Path, r.Header.Get("x-api-key")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true,"results":[{"title":"Paper","url":"https://arxiv.org/abs/1","content":"abcdefghij","source":"valyu/valyu-arxiv","relevance_score":0.9}]}`))
	}))
	defer srv.Close()

	v := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithTextLength(4))
	out, err := v.SearchAcademicSources(AcademicParams{Query: "transformers", StartDate: "2024-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/deepsearch" || gotKey != "k" {
		t.Fatalf("unexpected request: %s %s %s", gotMethod, gotPath, gotKey)
	}
	if gotBody["query"] != "transformers" || gotBody["search_type"] != "proprietary" || gotBody["start_date"] != "2024-01-01" ||
		gotBody["max_num_results"].(float64) != 10 || len(gotBody["included_sources"].([]interface{})) != 4 {
		t.Fatalf("unexpected body: %v", gotBody)
	}
	var res []map[string]interface{}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0]["content"] != "abcd..." || res[0]["relevance_score"].(float64) != 0.9 {
		t.Fatalf("unexpected output: %s", out)
	}
	if len(v.GetMethods()) != 3 {
		t.Fatalf("expected 3 methods")
	}
}

func TestValyuErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":"insufficient credits"}`))
	}))
	defer srv.Close()
	v := New(WithAPIKey("k"), WithBaseURL(srv.URL))
	if out, _ := v.SearchWeb(WebParams{Query: "x"}); out != "Error: insufficient credits" {
		t.Fatalf("unexpected: %v", out)
	}
	if out, _ := v.SearchWithinPaper(PaperParams{PaperURL: "arxiv.org/1", Query: "x"}); out != "Error: Invalid paper URL format" {
		t.Fatalf("unexpected: %v", out)
	}
}

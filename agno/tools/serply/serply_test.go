package serply

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSerply(t *testing.T) {
	var gotPath, gotKey, gotUA, gotQ, gotNum string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey, gotUA = r.URL.Path, r.Header.Get("X-Api-Key"), r.Header.Get("User-Agent")
		gotQ, gotNum = r.URL.Query().Get("q"), r.URL.Query().Get("num")
		switch r.URL.Path {
		case "/search/":
			_, _ = w.Write([]byte(`{"results":[{"position":1,"title":"Go","link":"https://go.dev","description":"d"}],"related_searches":["golang"]}`))
		case "/news/":
			_, _ = w.Write([]byte(`{"entries":[{"title":"A","link":"a","source":{"title":"S"},"published":"p"},{"title":"B"},{"title":"C"}]}`))
		}
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL), WithAll())
	out, err := s.SearchWeb(SearchParams{Query: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/" || gotKey != "k" || gotUA != "agno" || gotQ != "golang" || gotNum != "10" {
		t.Fatalf("unexpected request: %s %s %s %s %s", gotPath, gotKey, gotUA, gotQ, gotNum)
	}
	var web struct {
		Results []map[string]interface{} `json:"results"`
		Related []string                 `json:"related_searches"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &web); err != nil || len(web.Results) != 1 || web.Results[0]["link"] != "https://go.dev" || web.Related[0] != "golang" {
		t.Fatalf("unexpected output: %s %v", out, err)
	}

	out, _ = s.SearchNews(SearchParams{Query: "golang", NumResults: 2})
	var news struct {
		News []map[string]interface{} `json:"news_results"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &news); err != nil || len(news.News) != 2 || news.News[0]["source"] != "S" || gotNum != "2" {
		t.Fatalf("unexpected news output: %s %v", out, err)
	}
	if len(s.GetMethods()) != 3 || len(New().GetMethods()) != 1 {
		t.Fatal("unexpected method registration")
	}
	if out, _ := New(WithAPIKey("k")).SearchWeb(SearchParams{}); out != `{"error":"Please provide a query to search for"}` {
		t.Fatalf("unexpected: %v", out)
	}
}

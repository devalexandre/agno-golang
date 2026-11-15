package searchapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchApi(t *testing.T) {
	var gotPath string
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query()
		switch r.URL.Query().Get("engine") {
		case "google":
			_, _ = w.Write([]byte(`{"organic_results":[{"position":1,"title":"Go","link":"https://go.dev","snippet":"s"}],"knowledge_graph":{"title":"Go"},"related_questions":[{"question":"q?","snippet":"a"}]}`))
		case "youtube":
			_, _ = w.Write([]byte(`{"videos":[{"id":"1","channel":{"title":"C"},"thumbnail":{"static":"t"}},{"id":"2"},{"id":"3"}]}`))
		}
	}))
	defer srv.Close()

	s := New(WithAPIKey("k"), WithBaseURL(srv.URL+"/api/v1/search"), WithAll())
	out, err := s.SearchGoogle(GoogleParams{Query: "golang", Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/search" || gotQuery["api_key"][0] != "k" || gotQuery["q"][0] != "golang" || gotQuery["num"][0] != "5" || gotQuery["hl"][0] != "en" {
		t.Fatalf("unexpected request: %s %v", gotPath, gotQuery)
	}
	var g struct {
		Organic []map[string]interface{} `json:"organic_results"`
		KG      map[string]interface{}   `json:"knowledge_graph"`
		RQ      []map[string]interface{} `json:"related_questions"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &g); err != nil || g.Organic[0]["link"] != "https://go.dev" || g.KG["title"] != "Go" || g.RQ[0]["question"] != "q?" {
		t.Fatalf("unexpected output: %s %v", out, err)
	}

	out, _ = s.SearchYoutube(YoutubeParams{Query: "golang", NumResults: 2})
	var y struct {
		Videos []map[string]interface{} `json:"video_results"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &y); err != nil || len(y.Videos) != 2 || y.Videos[0]["channel"] != "C" || y.Videos[0]["thumbnail"] != "t" {
		t.Fatalf("unexpected youtube output: %s %v", out, err)
	}
	if _, ok := gotQuery["num"]; ok {
		t.Fatal("youtube must not send num")
	}
	if len(s.GetMethods()) != 4 || len(New().GetMethods()) != 1 {
		t.Fatal("unexpected method registration")
	}
}

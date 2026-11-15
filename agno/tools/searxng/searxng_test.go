package searxng

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearxngNewsSearch(t *testing.T) {
	var gotPath, gotFormat, gotQ, gotCat, gotEngines string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotFormat = r.URL.Query().Get("format")
		gotQ = r.URL.Query().Get("q")
		gotCat = r.URL.Query().Get("categories")
		gotEngines = r.URL.Query().Get("engines")
		_, _ = w.Write([]byte(`{"query":"go","results":[{"url":"a"},{"url":"b"},{"url":"c"}]}`))
	}))
	defer srv.Close()

	s := New(WithHost(srv.URL), WithEngines("google", "bing"), WithFixedMaxResults(2))
	out, err := s.NewsSearch(SearchParams{Query: "go lang"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search" || gotFormat != "json" || gotQ != "go lang" || gotCat != "news" || gotEngines != "google,bing" {
		t.Fatalf("unexpected request: %s %s %s %s %s", gotPath, gotFormat, gotQ, gotCat, gotEngines)
	}
	var res struct {
		Results []map[string]string `json:"results"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 2 || res.Results[1]["url"] != "b" {
		t.Fatalf("unexpected output: %s", out)
	}
	if len(s.GetMethods()) != 8 {
		t.Fatalf("expected 8 methods, got %d", len(s.GetMethods()))
	}
}

func TestSearxngError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	out, err := New(WithHost(srv.URL)).SearchWeb(SearchParams{Query: "x"})
	if err != nil || !strings.HasPrefix(out.(string), "Error fetching results from searxng") {
		t.Fatalf("unexpected: %v %v", out, err)
	}
}

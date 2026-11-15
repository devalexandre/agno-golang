package tools

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newKeenableServer(t *testing.T, check func(r *http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		check(r)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/search"):
			_, _ = w.Write([]byte(`{"results":[
				{"title":"Go","url":"https://go.dev","snippet":"The Go\n  programming language","description":""},
				{"title":"NoURL","snippet":"skip"},
				{"title":"Second","url":"https://example.org","description":"desc only"}]}`))
		case strings.HasPrefix(r.URL.Path, "/v1/fetch"):
			_, _ = w.Write([]byte(`{"title":"Page","content":"# Hello"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestKeenableKeylessSearch(t *testing.T) {
	t.Setenv("KEENABLE_API_KEY", "")
	var gotPath, gotKey, gotTitle string
	var body map[string]string
	srv := newKeenableServer(t, func(r *http.Request) {
		gotPath, gotKey, gotTitle = r.URL.Path, r.Header.Get("X-API-Key"), r.Header.Get("X-Keenable-Title")
		_ = json.NewDecoder(r.Body).Decode(&body)
	})
	k := NewKeenableTools(WithKeenableBaseURL(srv.URL))
	if _, ok := k.GetMethods()["KeenableTools_FetchURLContent"]; ok {
		t.Fatal("fetch should be disabled by default")
	}
	out, err := k.WebSearch(KeenableSearchParams{Query: "golang"})
	if err != nil {
		t.Fatal(err)
	}
	s := out.(string)
	if gotPath != "/v1/search/public" || gotKey != "" || gotTitle != "Agno" {
		t.Fatalf("path=%q key=%q title=%q", gotPath, gotKey, gotTitle)
	}
	if body["query"] != "golang" || body["mode"] != "pro" {
		t.Fatalf("unexpected body %v", body)
	}
	if !strings.Contains(s, "### [Go](https://go.dev)\nThe Go programming language") ||
		!strings.Contains(s, "desc only") || strings.Contains(s, "NoURL") {
		t.Fatalf("unexpected markdown:\n%s", s)
	}
}

func TestKeenableKeyedSearchJSONAndLimit(t *testing.T) {
	var gotPath, gotKey string
	srv := newKeenableServer(t, func(r *http.Request) { gotPath, gotKey = r.URL.Path, r.Header.Get("X-API-Key") })
	k := NewKeenableTools(WithKeenableBaseURL(srv.URL), WithKeenableAPIKey("k1"), WithKeenableFormat("json"), WithKeenableMaxSnippetChars(6))
	out, _ := k.WebSearch(KeenableSearchParams{Query: "q", MaxResults: 1})
	if gotPath != "/v1/search" || gotKey != "k1" {
		t.Fatalf("path=%q key=%q", gotPath, gotKey)
	}
	var parsed struct {
		Results []struct{ Title, URL, Content string } `json:"results"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Results) != 1 || parsed.Results[0].Content != "The Go…" {
		t.Fatalf("unexpected results %+v", parsed.Results)
	}
}

func TestKeenableFetch(t *testing.T) {
	t.Setenv("KEENABLE_API_KEY", "")
	var gotPath, gotURL string
	srv := newKeenableServer(t, func(r *http.Request) { gotPath, gotURL = r.URL.Path, r.URL.Query().Get("url") })
	k := NewKeenableTools(WithKeenableBaseURL(srv.URL), WithKeenableAll())

	out, _ := k.FetchURLContent(KeenableFetchParams{URL: "https://8.8.8.8/page"})
	if out.(string) != "## Page\n\n# Hello" || gotPath != "/v1/fetch/public" || gotURL != "https://8.8.8.8/page" {
		t.Fatalf("out=%q path=%q url=%q", out, gotPath, gotURL)
	}

	orig := keenableLookupIP
	defer func() { keenableLookupIP = orig }()
	keenableLookupIP = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("10.0.0.5")}, nil }
	for _, u := range []string{"file:///etc/passwd", "http://localhost/x", "http://127.0.0.1/", "http://[::ffff:192.168.0.1]/", "http://printer.local/", "http://intranet.example.com/"} {
		gotPath = ""
		out, _ := k.FetchURLContent(KeenableFetchParams{URL: u})
		if !strings.HasPrefix(out.(string), "Error: refusing") || gotPath != "" {
			t.Errorf("%s: expected refusal, got %q", u, out)
		}
	}
}

func TestKeenableErrorsAndBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"message":"slow down"}`))
	}))
	defer srv.Close()
	out, _ := NewKeenableTools(WithKeenableBaseURL(srv.URL)).WebSearch(KeenableSearchParams{Query: "x"})
	if out.(string) != "Keenable rate limit exceeded (429): slow down" {
		t.Fatalf("got %q", out)
	}
	out, _ = NewKeenableTools(WithKeenableBaseURL("http://example.com")).WebSearch(KeenableSearchParams{Query: "x"})
	if !strings.Contains(out.(string), "must be an https://") {
		t.Fatalf("plain http to non-loopback must be refused, got %q", out)
	}
}

package jina

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJinaReadURLAndSearch(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotTimeout, gotRespondWith, gotLinks string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotAuth, gotTimeout = r.Header.Get("Authorization"), r.Header.Get("X-Timeout")
		gotRespondWith, gotLinks = r.Header.Get("X-Respond-With"), r.Header.Get("X-With-Links-Summary")
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
		}
		_, _ = w.Write([]byte(`{"code":200,"data":{"content":"` + strings.Repeat("x", 100) + `"}}`))
	}))
	defer srv.Close()

	j := New(WithAPIKey("k"), WithBaseURL(srv.URL+"/r/"), WithSearchURL(srv.URL+"/s/"), WithMaxContentLength(20),
		WithTimeout(15*time.Second), WithAll(), WithSearchQueryContent(false))
	out, err := j.ReadURL(ReadURLParams{URL: "https://go.dev"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/r/https://go.dev" || gotAuth != "Bearer k" || gotTimeout != "15" || gotLinks != "true" {
		t.Fatalf("unexpected read request: %s %s %s %s %s", gotMethod, gotPath, gotAuth, gotTimeout, gotLinks)
	}
	if s := out.(string); !strings.HasSuffix(s, "... (content truncated)") || !strings.HasPrefix(s, `{"code":200`) {
		t.Fatalf("unexpected output: %s", s)
	}

	if _, err := j.SearchQuery(SearchQueryParams{Query: "golang"}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/s/" || gotBody["q"] != "golang" || gotRespondWith != "no-content" {
		t.Fatalf("unexpected search request: %s %s %v %s", gotMethod, gotPath, gotBody, gotRespondWith)
	}
	if len(j.GetMethods()) != 2 || len(New().GetMethods()) != 1 {
		t.Fatalf("unexpected method registration")
	}
}

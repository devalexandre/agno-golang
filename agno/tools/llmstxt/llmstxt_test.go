package llmstxt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const index = `# Project

> Docs for the project.

## Docs

- [Quickstart](/docs/quickstart.html): Get started fast
- [API](https://other.example/api.md)

## Optional

- [Changelog](/changelog.md)
`

func TestLLMsTxt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/llms.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(index))
		case "/docs/quickstart.html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><nav>Nav</nav><main><h1>Quickstart</h1><p>Install it.</p></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	l := New(WithSkipOptional(true))
	out, err := l.GetLlmsTxtIndex(URLParams{URL: srv.URL + "/llms.txt"})
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Overview string  `json:"overview"`
		Pages    []Entry `json:"pages"`
		Total    int     `json:"total_pages"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if res.Total != 2 || res.Pages[0].URL != srv.URL+"/docs/quickstart.html" || res.Pages[0].Description != "Get started fast" ||
		res.Pages[0].Section != "Docs" || !strings.Contains(res.Overview, "Docs for the project") {
		t.Fatalf("unexpected index: %s", out)
	}

	page, _ := l.ReadLlmsTxtUrl(URLParams{URL: res.Pages[0].URL})
	if page != "Quickstart\nInstall it." {
		t.Fatalf("unexpected page: %q", page)
	}
	if miss, _ := l.ReadLlmsTxtUrl(URLParams{URL: srv.URL + "/nope"}); !strings.HasPrefix(miss.(string), "Failed to fetch content") {
		t.Fatalf("unexpected: %v", miss)
	}

	restricted := New(WithAllowedHosts("docs.example.com"))
	if out, _ := restricted.ReadLlmsTxtUrl(URLParams{URL: srv.URL + "/llms.txt"}); !strings.HasPrefix(out.(string), "Host is not in allowed_hosts") {
		t.Fatalf("unexpected: %v", out)
	}
}

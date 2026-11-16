// HTTPPageFetcher: turn a list of URLs into whole-page text with provenance
// (extractor, attempts, per-page error) — the seam used by SitemapReader.
// HTML is reduced to its main content, text/markdown is kept, PDFs are read.
//
// Offline: a local httptest server provides the pages. Set PublicOnly (and
// AllowedHosts) when the URLs come from untrusted input.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><head><title>Article</title></head><body><nav>skip</nav><article><h1>Go agents</h1><p>Readers feed knowledge.</p></article></body></html>")
	})
	mux.HandleFunc("/notes.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Notes\n- one\n- two")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := reader.NewHTTPPageFetcher()
	f.Concurrency = 4
	urls := []string{srv.URL + "/article", srv.URL + "/notes.md", srv.URL + "/missing", "https://not-allowed.example/x"}
	for _, p := range f.FetchMany(context.Background(), urls, []string{"127.0.0.1"}) {
		if !p.OK() {
			fmt.Printf("%s -> error=%q attempts=%v\n", p.URL, p.Error, p.Attempts)
			continue
		}
		fmt.Printf("%s -> title=%q extractor=%s\n%s\n\n", p.URL, p.Title, p.Extractor, p.Content)
	}
}

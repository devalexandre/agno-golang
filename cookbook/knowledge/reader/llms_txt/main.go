// LLMsTxtReader: read an llms.txt (https://llmstxt.org) and the docs it links.
//
// Offline by default: a local httptest server serves a sample llms.txt and two
// linked pages. Pass a real URL to read a live site, e.g.
//
//	go run ./cookbook/knowledge/reader/llms_txt https://docs.agno.com/llms.txt
//
// For URLs from untrusted input enable WithPublicOnly(true) (SSRF-safe client).
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	source := ""
	r := reader.NewLLMsTxtReader()
	r.MaxURLs = 10
	r.SkipOptional = true
	if len(os.Args) > 1 {
		source = os.Args[1]
		r.WithPublicOnly(true)
	} else {
		srv := sampleSite()
		defer srv.Close()
		source = srv.URL + "/llms.txt"
	}

	docs, err := r.Read(context.Background(), source)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		content := d.Content
		if len(content) > 120 {
			content = content[:120] + "..."
		}
		fmt.Printf("[%v] %s (%v)\n  %q\n", d.Metadata["type"], d.Name, d.Metadata["url"], content)
	}
}

func sampleSite() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/llms.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "# Demo\n\n> Demo project docs.\n\n## Docs\n\n- [Quickstart](/quickstart.md): Install and run\n- [Agents](/agents.html): Build agents\n\n## Optional\n\n- [Changelog](/changelog.md)\n")
	})
	mux.HandleFunc("/quickstart.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Quickstart\n\ngo get github.com/devalexandre/agno-golang")
	})
	mux.HandleFunc("/agents.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><nav>menu</nav><main><h1>Agents</h1><p>Agents use models and tools.</p></main></body></html>")
	})
	return httptest.NewServer(mux)
}

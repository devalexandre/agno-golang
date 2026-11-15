// SitemapReader: discover a site's sitemap (robots.txt, /sitemap.xml, indexes,
// .xml.gz) and load one whole-page document per page, with its canonical URL.
//
// Offline by default: a local httptest server serves robots.txt, a sitemap and
// pages. Pass a real site to read it live (PublicOnly is then enabled):
//
//	go run ./cookbook/knowledge/reader/sitemap https://example.com/
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
	r := reader.NewSitemapReader()
	r.MaxPages = 20
	r.SourceHeader = true // prepend "# <title>\nSource: <url>" to each page

	var source string
	if len(os.Args) > 1 {
		source = os.Args[1]
		r.WithPublicOnly(true)
	} else {
		srv := sampleSite()
		defer srv.Close()
		source = srv.URL + "/"
	}

	pages, sitemapURL, incomplete := r.Discover(context.Background(), source)
	fmt.Printf("sitemap=%s pages=%d incomplete=%v\n\n", sitemapURL, len(pages), incomplete)

	docs, err := r.Read(context.Background(), source)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		if e, ok := d.Metadata["error"]; ok {
			fmt.Printf("%s -> error %v\n", d.Name, e)
			continue
		}
		fmt.Printf("%s (%v)\n%s\n\n", d.Name, d.Metadata["title"], d.Content)
	}
}

func sampleSite() *httptest.Server {
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "User-agent: *\nSitemap: %s/sitemap.xml\n", srv.URL)
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`+
			`<url><loc>%[1]s/</loc></url><url><loc>%[1]s/about</loc></url><url><loc>%[1]s/about/</loc></url></urlset>`, srv.URL)
	})
	page := func(title, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><head><title>%s</title></head><body><header>nav</header><main><p>%s</p></main></body></html>", title, body)
		}
	}
	mux.HandleFunc("/about", page("About", "We build agents in Go."))
	mux.HandleFunc("/", page("Home", "Welcome to the demo site."))
	srv = httptest.NewServer(mux)
	return srv
}

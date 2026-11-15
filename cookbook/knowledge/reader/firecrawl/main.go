// FirecrawlReader: scrape a page (or crawl a site) as markdown via the
// Firecrawl API.
//
// Requires FIRECRAWL_API_KEY. Usage:
//
//	FIRECRAWL_API_KEY=fc-... go run ./cookbook/knowledge/reader/firecrawl [url] [crawl]
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	if os.Getenv("FIRECRAWL_API_KEY") == "" {
		log.Fatal("set FIRECRAWL_API_KEY")
	}
	url := "https://github.com/devalexandre/agno-golang"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	r := reader.NewFirecrawlReader("") // empty key -> FIRECRAWL_API_KEY
	r.Params = map[string]any{"onlyMainContent": true}
	if len(os.Args) > 2 && os.Args[2] == "crawl" {
		r.Mode = reader.FirecrawlCrawl
		r.Params = map[string]any{"limit": 5}
	}
	docs, err := r.Read(context.Background(), url)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		content := d.Content
		if len(content) > 300 {
			content = content[:300] + "..."
		}
		fmt.Printf("== %s\n%s\n\n", d.ID, content)
	}
}

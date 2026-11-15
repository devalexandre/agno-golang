// WebSearchReader: search DuckDuckGo (keyless HTML endpoint) for a topic and
// read the result pages into documents.
//
// Requires network access (no API key). Usage:
//
//	go run ./cookbook/knowledge/reader/web_search "agno agent framework"
//
// Any search backend can be used by implementing reader.WebSearcher.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	query := "agno agent framework"
	if len(os.Args) > 1 {
		query = strings.Join(os.Args[1:], " ")
	}
	r := reader.NewWebSearchReader().WithPublicOnly(true) // result URLs are untrusted
	r.MaxResults = 3
	r.DelayBetweenRequests = time.Second
	docs, err := r.Read(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		content := d.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		fmt.Printf("== %s\n%s\n%s\n\n", d.Name, d.Metadata["url"], content)
	}
}

// TavilyReader: extract a page's content (markdown or text) with Tavily's
// Extract API.
//
// Requires TAVILY_API_KEY. Usage:
//
//	TAVILY_API_KEY=tvly-... go run ./cookbook/knowledge/reader/tavily [url]
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	if os.Getenv("TAVILY_API_KEY") == "" {
		log.Fatal("set TAVILY_API_KEY")
	}
	url := "https://github.com/devalexandre/agno-golang"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	r := reader.NewTavilyReader("") // empty key -> TAVILY_API_KEY
	r.ExtractFormat = "markdown"
	r.ExtractDepth = "basic" // "advanced" costs more credits but handles harder pages
	docs, err := r.Read(context.Background(), url)
	if err != nil {
		log.Fatal(err)
	}
	content := docs[0].Content
	if len(content) > 500 {
		content = content[:500] + "..."
	}
	fmt.Printf("== %s (%d chars)\n%s\n", docs[0].ID, len(docs[0].Content), content)
}

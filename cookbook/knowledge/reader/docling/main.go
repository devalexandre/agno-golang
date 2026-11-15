// DoclingReader: convert PDFs, Office files, images, HTML... through a
// docling-serve instance and get markdown (or text/json/yaml/html/doctags).
//
// Requires docling-serve:
//
//	docker run -p 5001:5001 quay.io/docling-project/docling-serve
//
// Env: DOCLING_URL (default http://localhost:5001), DOCLING_API_KEY (optional).
// Usage: go run ./cookbook/knowledge/reader/docling [url-or-file]
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	source := "https://arxiv.org/pdf/2408.09869"
	if len(os.Args) > 1 {
		source = os.Args[1]
	}
	r := reader.NewDoclingReader(os.Getenv("DOCLING_URL"))
	r.APIKey = os.Getenv("DOCLING_API_KEY")
	r.OutputFormat = "markdown"
	r.PublicOnly = true // the server fetches URLs itself: refuse private addresses
	docs, err := r.Read(context.Background(), source)
	if err != nil {
		log.Fatal(err)
	}
	content := docs[0].Content
	if len(content) > 800 {
		content = content[:800] + "..."
	}
	fmt.Printf("== %s (%d chars)\n%s\n", docs[0].Name, len(docs[0].Content), content)
}

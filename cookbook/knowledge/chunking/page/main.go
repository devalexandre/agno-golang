// PageMarkdownChunking: heading-aware chunks for documentation pages.
// Splits on H1/H2 (outside code fences), keeps newlines and fences intact,
// re-opens fences in oversized code blocks, and prefixes every chunk with the
// page context line and a "Title › Section" breadcrumb.
//
// Offline: chunks an embedded sample page. No env vars needed.
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/chunking"
)

const page = "# Knowledge (/docs/knowledge)\n\nKnowledge gives agents domain context at runtime.\n\n" +
	"## Readers\n\nReaders turn files and URLs into documents.\n\n### Formats\n\nPDF, Excel, PPTX, JSON...\n\n" +
	"## Example [#example]\n\n```go\n// # this is not a heading\ndocs, _ := reader.NewPDFReader().Read(ctx, \"file.pdf\")\n```\n"

func main() {
	doc := document.NewDocument(page)
	doc.ID = "knowledge-page"
	doc.Metadata["path"] = "/docs/knowledge" // drives the "section" and "kind" metadata

	chunks, err := chunking.NewPageMarkdownChunking(300, 2).Chunk(doc)
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range chunks {
		fmt.Printf("--- %s  breadcrumb=%q kind=%v\n%s\n\n", c.ID, c.Metadata["breadcrumb"], c.Metadata["kind"], c.Content)
	}
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println("intro:", chunking.PageIntro(page, 300))
}

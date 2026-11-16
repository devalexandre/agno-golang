// Chunking strategy factory: build any chunker by name (Python's
// ChunkingStrategyType values such as "FixedSizeChunker", or aliases like
// "fixed", "markdown", "page") with chunking.ParseStrategyType + NewStrategy.
// AgenticChunker needs a Model and SemanticChunker an Embedder in
// StrategyOptions; they are skipped here.
//
// Offline: chunks an embedded text with every model-free strategy.
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/chunking"
)

func main() {
	text := "# Title\n\n" + strings.Repeat("Agno agents read documents and answer questions. ", 12) +
		"\n\n## Section\n\n" + strings.Repeat("Chunking splits long text into pieces. ", 12)

	for _, name := range []string{"FixedSizeChunker", "recursive", "document", "markdown", "page", "code", "row"} {
		st, err := chunking.ParseStrategyType(name)
		if err != nil {
			log.Fatal(err)
		}
		strategy, err := chunking.NewStrategy(st, chunking.StrategyOptions{ChunkSize: 300, Overlap: 20, RowsPerChunk: 2})
		if err != nil {
			log.Fatal(err)
		}
		doc := document.NewDocument(text)
		doc.ID = "demo"
		chunks, err := strategy.Chunk(doc)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-18s -> %-16s %d chunk(s), first=%q\n", name, st, len(chunks), preview(chunks[0].Content))
	}
}

func preview(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 50 {
		return s[:50] + "..."
	}
	return s
}

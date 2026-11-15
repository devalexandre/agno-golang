// Example: embeddings through the LangDB AI gateway (OpenAI-compatible).
//
//	export LANGDB_API_KEY=...
//	export LANGDB_PROJECT_ID=...
//	go run ./cookbook/embedder/langdb
package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/embedder"
)

func main() {
	emb, err := embedder.NewLangDBEmbedder("", "", "text-embedding-3-small", 1536, "")
	if err != nil {
		log.Fatal(err)
	}
	vec, usage, err := emb.GetEmbeddingAndUsage("Agno routes embeddings through LangDB.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d dims, usage %v\n", emb.GetID(), len(vec), usage)
}

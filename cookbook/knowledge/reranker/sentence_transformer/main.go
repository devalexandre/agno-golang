// Example: reranking with a sentence-transformers cross-encoder served by
// Hugging Face Text Embeddings Inference (TEI) - the Go counterpart of Agno's
// SentenceTransformerReranker.
//
//	docker run -p 8081:80 ghcr.io/huggingface/text-embeddings-inference:cpu-1.5 \
//	  --model-id BAAI/bge-reranker-v2-m3
//	go run ./cookbook/knowledge/reranker/sentence_transformer
//
// Env: SENTENCE_TRANSFORMERS_RERANK_URL or TEI_RERANK_URL (default
// http://localhost:8081), TEI_API_KEY (optional).
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

func main() {
	r := reranker.NewSentenceTransformerReranker("", "BAAI/bge-reranker-v2-m3", 2)

	candidates := []*vectordb.SearchResult{
		{Document: &document.Document{Name: "carbonara", Content: "Roman pasta with eggs, pecorino and guanciale."}},
		{Document: &document.Document{Name: "tom-yum", Content: "Hot and sour Thai soup with lemongrass and shrimp."}},
		{Document: &document.Document{Name: "gazpacho", Content: "Cold Spanish tomato soup."}},
	}
	out, err := r.Rerank(context.Background(), "a spicy soup", candidates, 0)
	if err != nil {
		log.Fatal(err)
	}
	for _, res := range out {
		fmt.Printf("%.3f  %s\n", res.Score, res.Document.Name)
	}
}

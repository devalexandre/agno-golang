// Example: sentence-transformers embeddings served by Hugging Face Text
// Embeddings Inference (TEI) - the Go counterpart of Agno's
// SentenceTransformerEmbedder, which runs the model in-process in Python.
//
//	docker run -p 8080:80 ghcr.io/huggingface/text-embeddings-inference:cpu-1.5 \
//	  --model-id sentence-transformers/all-MiniLM-L6-v2
//	go run ./cookbook/embedder/sentence_transformer
//
// Env: SENTENCE_TRANSFORMERS_BASE_URL or TEI_BASE_URL (default
// http://localhost:8080), TEI_API_KEY (optional).
package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/embedder"
)

func main() {
	emb := embedder.NewSentenceTransformerEmbedder("", "sentence-transformers/all-MiniLM-L6-v2", 384)
	emb.NormalizeEmbeddings = true

	vec, err := emb.GetEmbedding("The quick brown fox jumps over the lazy dog.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d dims, first values %v\n", emb.GetID(), len(vec), vec[:3])
}

// Knowledge retrieval pipeline: HyDE query transformer + reranker.
//
// Search flow (mirrors agno-python Knowledge.search):
//
//  1. HyDE asks a model for a hypothetical answer and searches with it;
//  2. the vector db returns reranker.SearchLimit(n) candidates (x3 by default);
//  3. the reranker reorders them against the ORIGINAL question and the
//     result is trimmed to n.
//
// Requirements: Qdrant on localhost:6333 (docker run -p 6333:6333 -p 6334:6334
// qdrant/qdrant) and Ollama on localhost:11434 with the models below pulled.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/knowledge/querytransformer"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/qdrant"
)

func main() {
	ctx := context.Background()

	emb := embedder.NewOllamaEmbedder(
		embedder.WithOllamaModel("nomic-embed-text", 768),
		embedder.WithOllamaHost("http://localhost:11434"),
	)
	db, err := qdrant.NewQdrant(qdrant.QdrantConfig{
		Host:       "localhost",
		Port:       6334,
		Collection: "rerank_hyde_demo",
		Embedder:   emb,
		SearchType: vectordb.SearchTypeVector,
		Distance:   vectordb.DistanceCosine,
	})
	if err != nil {
		log.Fatalf("qdrant: %v", err)
	}

	kb := knowledge.NewBaseKnowledge("rerank_hyde_demo", db)
	docs := []document.Document{
		{ID: "1", Content: "Go was designed at Google in 2007 and released in 2009.", Metadata: map[string]interface{}{"updated_at": "2024-01-10"}},
		{ID: "2", Content: "Go 1.22 changed loop variable scoping so each iteration gets a new variable.", Metadata: map[string]interface{}{"updated_at": "2024-02-06"}},
		{ID: "3", Content: "Goroutines are lightweight threads managed by the Go runtime.", Metadata: map[string]interface{}{"updated_at": "2023-05-01"}},
		{ID: "4", Content: "Goroutines are cheap: thousands can run on a handful of OS threads.", Metadata: map[string]interface{}{"updated_at": "2023-05-02"}},
		{ID: "5", Content: "Channels let goroutines communicate by sending typed values.", Metadata: map[string]interface{}{"updated_at": "2025-03-12"}},
	}
	if err := kb.LoadDocuments(ctx, docs, true); err != nil {
		log.Fatalf("load: %v", err)
	}

	model, err := ollama.NewOllamaChat(models.WithID("llama3.2"))
	if err != nil {
		log.Fatalf("model: %v", err)
	}

	// Query transformer: search with a hypothetical answer. IncludeQuery keeps
	// the question's own words too (important for keyword/hybrid stores).
	hyde := querytransformer.NewHyDE(model)
	hyde.IncludeQuery = true
	kb.QueryTransformer = hyde

	// Reranker: blend relevance with freshness ("updated_at" metadata,
	// 30-day half-life, weight 0.3). Alternatives:
	//   reranker.NewCohereReranker("", "", 0)   hosted cross-encoder
	//   reranker.NewMMRReranker(nil, 0.5, 0)    diversity; needs a vector db that
	//                                           returns embeddings on search
	rec := reranker.NewRecencyReranker()
	rec.HalfLifeDays = 365
	kb.Reranker = rec

	question := "How does Go handle concurrency?"
	results, err := kb.Search(ctx, question, 3)
	if err != nil {
		log.Fatalf("search: %v", err)
	}
	fmt.Printf("Q: %s\n", question)
	for i, r := range results {
		fmt.Printf("%d. (%.3f) %s\n", i+1, r.Score, r.Document.Content)
	}
}

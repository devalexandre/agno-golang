// Example: reranking search results with the AWS Bedrock Rerank API
// (Cohere Rerank 3.5 or Amazon Rerank 1.0).
//
//	export AWS_REGION=us-west-2
//	export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...   # or AWS_PROFILE / SSO
//	go run ./cookbook/knowledge/reranker/aws_bedrock
//
// Amazon Rerank 1.0 is not available in us-east-1; use Cohere Rerank 3.5 there.
// In a RAG pipeline set it as BaseKnowledge.Reranker instead of calling it directly.
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
	r := reranker.NewCohereBedrockReranker("", 2) // region from AWS_REGION

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

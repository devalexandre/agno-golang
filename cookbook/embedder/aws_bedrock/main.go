// Example: AWS Bedrock embeddings (Cohere Embed v3/v4 and Amazon Titan).
//
//	export AWS_REGION=us-east-1
//	export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...   # or AWS_PROFILE / SSO
//	go run ./cookbook/embedder/aws_bedrock
//
// The models must be enabled in the Bedrock console ("Model access").
package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/embedder"
)

func main() {
	// Cohere Embed v3 (1024 dims) - the Agno default.
	cohere, err := embedder.NewAwsBedrockEmbedder(embedder.AwsBedrockOptions{
		Model:     embedder.BedrockCohereEmbedMultilingualV3,
		InputType: "search_document",
	})
	if err != nil {
		log.Fatal(err)
	}
	vec, err := cohere.GetEmbedding("Agno is a framework for building AI agents.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d dims, first values %v\n", cohere.GetID(), len(vec), vec[:3])

	// Amazon Titan Text Embeddings v2 with a reduced output dimension.
	titan, err := embedder.NewAwsBedrockEmbedder(embedder.AwsBedrockOptions{
		Model:           embedder.BedrockTitanEmbedTextV2,
		OutputDimension: 512,
	})
	if err != nil {
		log.Fatal(err)
	}
	vec, usage, err := titan.GetEmbeddingAndUsage("Agno is a framework for building AI agents.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %d dims, usage %v\n", titan.GetID(), len(vec), usage)
}

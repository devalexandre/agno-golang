// Example: listing and invoking AWS Lambda functions with the AWS Lambda toolkit.
//
// Uses the default AWS credential chain (env vars, ~/.aws, SSO, IMDS...).
//
//	go run ./cookbook/tools/awslambda
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/awslambda"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	lambdaTools := awslambda.New(awslambda.WithRegion("us-east-1"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Lambda Operator",
		Model:         model,
		Instructions:  "You help operate AWS Lambda functions.",
		Tools:         []toolkit.Tool{lambdaTools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List my Lambda functions and invoke the first one with an empty payload.", true, true)
}

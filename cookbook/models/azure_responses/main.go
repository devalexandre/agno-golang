// Example: using Azure OpenAI through the Responses API with agno-golang.
//
// API key: set AZURE_OPENAI_API_KEY (or AZURE_OPENAI_AD_TOKEN) and
// AZURE_OPENAI_ENDPOINT (e.g. https://my-resource.openai.azure.com).
// Model: your deployment name, from AZURE_OPENAI_DEPLOYMENT_NAME (example: gpt-5-mini).
// api-version: OPENAI_API_VERSION or 2025-04-01-preview by default.
//
//	go run ./cookbook/models/azure_responses
package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models/azure"
	"github.com/devalexandre/agno-golang/agno/models/openai/responses"
)

func main() {
	ctx := context.Background()

	deployment := os.Getenv(azure.EnvDeployment)
	if deployment == "" {
		deployment = "gpt-5-mini"
	}

	reasoning := true // deployment names are arbitrary; declare it explicitly
	model, err := azure.NewAzureResponses(azure.ResponsesConfig{
		Deployment:       deployment,
		IsReasoningModel: &reasoning,
		Responses: responses.Config{
			ReasoningEffort: "low",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Azure Responses Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

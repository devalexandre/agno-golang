// Example: ScrapeGraph Agent agent using the scrapegraph toolkit.
//
// Requires SGAI_API_KEY.
//
//	go run ./cookbook/tools/scrapegraph
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/scrapegraph"
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

	tools := scrapegraph.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "ScrapeGraph Agent",
		Model:         model,
		Instructions:  "Use ScrapeGraph to extract structured data from web pages.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Extract the product names and prices from https://example.com.", true, true)
}

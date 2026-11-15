// Example: web search with the Brave Search toolkit.
//
// Requires BRAVE_API_KEY.
//
//	go run ./cookbook/tools/bravesearch
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/bravesearch"
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

	brave := bravesearch.New(bravesearch.WithFixedMaxResults(5))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Brave Researcher",
		Model:         model,
		Instructions:  "Search the web with Brave and cite the URLs you used.",
		Tools:         []toolkit.Tool{brave},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What is new in the latest Go release?", true, true)
}

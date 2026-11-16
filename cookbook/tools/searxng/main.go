// Example: web search with the SearXNG toolkit.
//
// Requires a SearXNG instance with JSON output enabled (SEARXNG_HOST, e.g. http://localhost:8080).
//
//	go run ./cookbook/tools/searxng
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/searxng"
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

	tk := searxng.New(searxng.WithHost("http://localhost:8080"), searxng.WithFixedMaxResults(5))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "SearXNG Researcher",
		Model:         model,
		Instructions:  "Search the web with SearXNG and cite the URLs you used.",
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What is new in the latest Go release?", true, true)
}

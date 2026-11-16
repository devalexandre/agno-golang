// Example: keyless web search with KeenableTools.
//
// Keenable works without an API key (public endpoints). Set KEENABLE_API_KEY to
// use the authenticated endpoints with higher rate limits.
//
//	go run ./cookbook/tools/keenable
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
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

	// Search is enabled by default; WithKeenableAll also enables page fetching.
	keenable := tools.NewKeenableTools(tools.WithKeenableAll(), tools.WithKeenableMaxResults(5))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Web Researcher",
		Model:         model,
		Instructions:  "Search the web with Keenable and cite the URLs you used. Fetch a page when snippets are not enough.",
		Tools:         []toolkit.Tool{keenable},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What are the latest stable Go releases and their headline features?", true, true)
}

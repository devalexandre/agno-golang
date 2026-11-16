// Example: scrape and search the web with the Anakin toolkit.
//
// Requires ANAKIN_API_KEY.
//
//	go run ./cookbook/tools/anakin
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/anakin"
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

	// ScrapeUrl is enabled by default; also enable web search.
	tools := anakin.New(anakin.WithEnableSearch(true))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Anakin Researcher",
		Model:         model,
		Instructions:  "Use Anakin to search and scrape the web. Cite the URLs you used.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Search for the latest Go release notes and summarize the highlights from the top result.", true, true)
}

// Example: Bright Data Agent agent using the brightdata toolkit.
//
// Requires BRIGHT_DATA_API_KEY.
//
//	go run ./cookbook/tools/brightdata
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/brightdata"
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

	tools := brightdata.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Bright Data Agent",
		Model:         model,
		Instructions:  "Use Bright Data to search and scrape the web.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Search Google for Go 1.24 release notes and summarize the top result.", true, true)
}

// Example: Sofya Researcher agent using the sofya toolkit.
//
// Requires SOFYA_API_KEY.
//
//	go run ./cookbook/tools/sofya
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/sofya"
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

	tools := sofya.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Sofya Researcher",
		Model:         model,
		Instructions:  "Use Sofya to search, extract and research the web with citations.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Summarize recent developments in WebAssembly.", true, true)
}

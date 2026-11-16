// Example: manage Browserbase cloud browser sessions.
//
// Requires BROWSERBASE_API_KEY and BROWSERBASE_PROJECT_ID.
//
//	go run ./cookbook/tools/browserbase
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/browserbase"
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

	bb := browserbase.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Browserbase Manager",
		Model:         model,
		Instructions:  "Manage Browserbase cloud browser sessions. Report session ids and connect URLs.",
		Tools:         []toolkit.Tool{bb},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Create a browser session, show its details, list running sessions, then close it.", true, true)
}

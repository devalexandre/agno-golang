// Example: messaging Webex rooms with the Webex toolkit.
//
// Requires WEBEX_ACCESS_TOKEN.
//
//	go run ./cookbook/tools/webex
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/webex"
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

	wx := webex.New()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Webex Assistant",
		Model:         model,
		Instructions:  "You help users communicate in Webex rooms.",
		Tools:         []toolkit.Tool{wx},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List my Webex rooms, then send 'Hello from Agno Go!' to the first one.", true, true)
}

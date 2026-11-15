// Example: using Nexus with agno-golang.
//
// No API key by default: run a Nexus router on http://localhost:8000.
// Default model: openai/gpt-4
//
//	go run ./cookbook/models/nexus
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/nexus"
)

func main() {
	ctx := context.Background()

	model, err := nexus.NewNexusChat(
		models.WithID(nexus.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Nexus Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

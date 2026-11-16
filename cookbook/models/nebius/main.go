// Example: using Nebius Token Factory with agno-golang.
//
// API key: set NEBIUS_API_KEY (export NEBIUS_API_KEY=...).
// Default model: openai/gpt-oss-20b
//
//	go run ./cookbook/models/nebius
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/nebius"
)

func main() {
	ctx := context.Background()

	model, err := nebius.NewNebiusChat(
		models.WithID(nebius.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Nebius Token Factory Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

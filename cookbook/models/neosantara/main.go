// Example: using Neosantara with agno-golang.
//
// API key: set NEOSANTARA_API_KEY (export NEOSANTARA_API_KEY=...).
// Default model: grok-4.1-fast-non-reasoning
//
//	go run ./cookbook/models/neosantara
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/neosantara"
)

func main() {
	ctx := context.Background()

	model, err := neosantara.NewNeosantaraChat(
		models.WithID(neosantara.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Neosantara Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

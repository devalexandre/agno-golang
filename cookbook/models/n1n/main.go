// Example: using N1N with agno-golang.
//
// API key: set N1N_API_KEY (export N1N_API_KEY=...).
// Default model: gpt-4o
//
//	go run ./cookbook/models/n1n
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/n1n"
)

func main() {
	ctx := context.Background()

	model, err := n1n.NewN1NChat(
		models.WithID(n1n.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "N1N Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

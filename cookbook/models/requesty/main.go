// Example: using Requesty with agno-golang.
//
// API key: set REQUESTY_API_KEY (export REQUESTY_API_KEY=...).
// Default model: openai/gpt-4.1
//
//	go run ./cookbook/models/requesty
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/requesty"
)

func main() {
	ctx := context.Background()

	model, err := requesty.NewRequestyChat(
		models.WithID(requesty.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Requesty Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

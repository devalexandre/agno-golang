// Example: using SambaNova with agno-golang.
//
// API key: set SAMBANOVA_API_KEY (export SAMBANOVA_API_KEY=...).
// Default model: Meta-Llama-3.1-8B-Instruct
//
//	go run ./cookbook/models/sambanova
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/sambanova"
)

func main() {
	ctx := context.Background()

	model, err := sambanova.NewSambanovaChat(
		models.WithID(sambanova.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "SambaNova Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

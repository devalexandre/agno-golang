// Example: using Cerebras with agno-golang.
//
// API key: set CEREBRAS_API_KEY (export CEREBRAS_API_KEY=...).
// Default model: gpt-oss-120b
//
//	go run ./cookbook/models/cerebras
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/cerebras"
)

func main() {
	ctx := context.Background()

	model, err := cerebras.NewCerebrasChat(
		models.WithID(cerebras.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Cerebras Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

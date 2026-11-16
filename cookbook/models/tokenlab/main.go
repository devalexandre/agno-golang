// Example: using TokenLab with agno-golang.
//
// API key: set TOKENLAB_API_KEY (export TOKENLAB_API_KEY=...).
// Default model: gpt-5.4-mini
//
//	go run ./cookbook/models/tokenlab
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/tokenlab"
)

func main() {
	ctx := context.Background()

	model, err := tokenlab.NewTokenLabChat(
		models.WithID(tokenlab.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "TokenLab Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

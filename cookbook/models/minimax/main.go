// Example: using MiniMax with agno-golang.
//
// API key: set MINIMAX_API_KEY (export MINIMAX_API_KEY=...).
// Default model: MiniMax-M3
//
//	go run ./cookbook/models/minimax
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/minimax"
)

func main() {
	ctx := context.Background()

	model, err := minimax.NewMiniMaxChat(
		models.WithID(minimax.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "MiniMax Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

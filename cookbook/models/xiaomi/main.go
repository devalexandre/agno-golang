// Example: using Xiaomi MiMo with agno-golang.
//
// API key: set MIMO_API_KEY (export MIMO_API_KEY=...).
// Default model: mimo-v2.5-pro
//
//	go run ./cookbook/models/xiaomi
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/xiaomi"
)

func main() {
	ctx := context.Background()

	model, err := xiaomi.NewMiMoChat(
		models.WithID(xiaomi.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Xiaomi MiMo Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

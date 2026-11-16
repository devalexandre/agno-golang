// Example: using Synthorai with agno-golang.
//
// API key: set SYNTHORAI_API_KEY (export SYNTHORAI_API_KEY=...).
// Default model: claude-opus-5
//
//	go run ./cookbook/models/synthorai
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/synthorai"
)

func main() {
	ctx := context.Background()

	model, err := synthorai.NewSynthoraiChat(
		models.WithID(synthorai.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Synthorai Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

// Example: using Inception (Mercury) with agno-golang.
//
// API key: set INCEPTION_API_KEY (export INCEPTION_API_KEY=...).
// Default model: mercury-2
//
//	go run ./cookbook/models/inception
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/inception"
)

func main() {
	ctx := context.Background()

	model, err := inception.NewInceptionChat(
		models.WithID(inception.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Inception (Mercury) Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

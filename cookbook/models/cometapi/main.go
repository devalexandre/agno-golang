// Example: using CometAPI with agno-golang.
//
// API key: set COMETAPI_KEY (export COMETAPI_KEY=...).
// Default model: gpt-5-mini
//
//	go run ./cookbook/models/cometapi
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/cometapi"
)

func main() {
	ctx := context.Background()

	model, err := cometapi.NewCometAPIChat(
		models.WithID(cometapi.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "CometAPI Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

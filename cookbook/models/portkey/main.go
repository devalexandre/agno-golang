// Example: using Portkey with agno-golang.
//
// API key: set PORTKEY_API_KEY and PORTKEY_VIRTUAL_KEY.
// Default model: gpt-4o-mini
//
//	go run ./cookbook/models/portkey
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/portkey"
)

func main() {
	ctx := context.Background()

	model, err := portkey.NewPortkeyChat(
		models.WithID(portkey.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Portkey Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

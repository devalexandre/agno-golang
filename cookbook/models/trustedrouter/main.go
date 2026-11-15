// Example: using TrustedRouter with agno-golang.
//
// API key: set TRUSTEDROUTER_API_KEY (export TRUSTEDROUTER_API_KEY=...).
// Default model: trustedrouter/zdr
//
//	go run ./cookbook/models/trustedrouter
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/trustedrouter"
)

func main() {
	ctx := context.Background()

	model, err := trustedrouter.NewTrustedRouterChat(
		models.WithID(trustedrouter.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "TrustedRouter Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

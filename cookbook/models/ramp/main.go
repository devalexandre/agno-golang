// Example: using Ramp Router with agno-golang.
//
// API key: set RAMP_ROUTER_API_KEY (export RAMP_ROUTER_API_KEY=...).
// Default model: gpt-5.6-luna
//
//	go run ./cookbook/models/ramp
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ramp"
)

func main() {
	ctx := context.Background()

	model, err := ramp.NewRampRouter(
		models.WithID(ramp.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Ramp Router Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Write a haiku about routing.", true, true)
}

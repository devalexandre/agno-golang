// Example: using Vercel v0 with agno-golang.
//
// API key: set V0_API_KEY (export V0_API_KEY=...).
// Default model: v0-1.0-md
//
//	go run ./cookbook/models/vercel
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/vercel"
)

func main() {
	ctx := context.Background()

	model, err := vercel.NewV0Chat(
		models.WithID(vercel.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Vercel v0 Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Write a minimal React button component.", true, true)
}

// Example: using DeepInfra with agno-golang.
//
// API key: set DEEPINFRA_API_KEY (export DEEPINFRA_API_KEY=...).
// Default model: meta-llama/Llama-2-70b-chat-hf
//
//	go run ./cookbook/models/deepinfra
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/deepinfra"
)

func main() {
	ctx := context.Background()

	model, err := deepinfra.NewDeepInfraChat(
		models.WithID(deepinfra.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "DeepInfra Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

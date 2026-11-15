// Example: using InternLM with agno-golang.
//
// API key: set INTERNLM_API_KEY (export INTERNLM_API_KEY=...).
// Default model: internlm2.5-latest
//
//	go run ./cookbook/models/internlm
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/internlm"
)

func main() {
	ctx := context.Background()

	model, err := internlm.NewInternLMChat(
		models.WithID(internlm.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "InternLM Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

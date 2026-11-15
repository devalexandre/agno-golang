// Example: using Llmman with agno-golang.
//
// No API key: start Llmman on http://127.0.0.1:17434.
// Default model: qwen3:0.6b-q4_K_M
//
//	go run ./cookbook/models/llmman
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/llmman"
)

func main() {
	ctx := context.Background()

	model, err := llmman.NewLlmmanChat(
		models.WithID(llmman.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Llmman Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

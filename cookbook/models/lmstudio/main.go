// Example: using LM Studio with agno-golang.
//
// No API key: start the LM Studio server on http://127.0.0.1:1234.
// Default model: qwen2.5-7b-instruct-1m
//
//	go run ./cookbook/models/lmstudio
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/lmstudio"
)

func main() {
	ctx := context.Background()

	model, err := lmstudio.NewLMStudioChat(
		models.WithID(lmstudio.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "LM Studio Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

// Example: MiniMaxTools video generation. Requires MINIMAX_API_KEY.
//
//	go run ./cookbook/tools/minimax
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/minimax"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	tk := minimax.NewMiniMaxTools()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(), Model: model, Tools: []toolkit.Tool{tk}, ShowToolsCall: true, Markdown: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	ag.PrintResponse("Generate a 5 second video of waves crashing on a beach.", true, true)
}

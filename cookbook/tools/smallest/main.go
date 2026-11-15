// Example: SmallestTools text-to-speech. Requires SMALLEST_API_KEY.
//
//	go run ./cookbook/tools/smallest
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/smallest"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	tk := smallest.NewSmallestTools(smallest.WithTargetDirectory("tmp"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(), Model: model, Tools: []toolkit.Tool{tk}, ShowToolsCall: true, Markdown: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	ag.PrintResponse("List a few voices, then say 'Welcome to Agno' with the default voice.", true, true)
}

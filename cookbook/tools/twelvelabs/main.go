// Example: TwelveLabsTools video understanding. Requires TWELVELABS_API_KEY.
//
//	go run ./cookbook/tools/twelvelabs
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/twelvelabs"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	tk := twelvelabs.NewTwelveLabsTools()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(), Model: model, Tools: []toolkit.Tool{tk}, ShowToolsCall: true, Markdown: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	ag.PrintResponse("Summarize this video: https://example.com/sample.mp4", true, true)
}

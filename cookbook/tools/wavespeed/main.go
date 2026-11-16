// Example: WaveSpeedTools image/video generation. Requires WAVESPEED_API_KEY.
//
//	go run ./cookbook/tools/wavespeed
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/wavespeed"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	tk := wavespeed.NewWaveSpeedTools()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(), Model: model, Tools: []toolkit.Tool{tk}, ShowToolsCall: true, Markdown: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	ag.PrintResponse("Generate an image of a futuristic city skyline.", true, true)
}

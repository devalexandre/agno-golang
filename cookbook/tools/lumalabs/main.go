// Example: lumalabs toolkit with an agent.
//
// Requires LUMAAI_API_KEY.
//
//	go run ./cookbook/tools/lumalabs
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/lumalabs"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	tk := lumalabs.NewLumaLabTools()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Model:         model,
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Generate a 16:9 video of waves crashing on a beach.", true, true)
}

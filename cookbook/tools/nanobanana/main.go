// Example: nanobanana toolkit with an agent.
//
// Requires GOOGLE_API_KEY.
//
//	go run ./cookbook/tools/nanobanana
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/nanobanana"
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

	tk, err := nanobanana.NewNanoBananaTools(nanobanana.WithAspectRatio("16:9"))
	if err != nil {
		log.Fatal(err)
	}

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

	ag.PrintResponse("Create an image of a banana wearing sunglasses.", true, true)
}

// Example: Apify Agent agent using the apify toolkit.
//
// Requires APIFY_API_TOKEN.
//
//	go run ./cookbook/tools/apify
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/apify"
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

	tools := apify.New(apify.WithActors("apify/rag-web-browser"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Apify Agent",
		Model:         model,
		Instructions:  "Use Apify Actors to browse the web and answer with sources.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What are the latest Agno framework features?", true, true)
}

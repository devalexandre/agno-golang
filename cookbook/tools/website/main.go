// Example: read a website with WebsiteTools.
//
//	go run ./cookbook/tools/website
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/website"
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

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Website Reader",
		Model:         model,
		Instructions:  "Read websites and answer questions about their content.",
		Tools:         []toolkit.Tool{website.New(website.WithMaxLinks(3))},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Read https://go.dev and describe what Go is.", true, true)
}

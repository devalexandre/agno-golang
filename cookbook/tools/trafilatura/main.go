// Example: extract the main text and metadata of web pages.
//
//	go run ./cookbook/tools/trafilatura
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/trafilatura"
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

	extractor := trafilatura.New(
		trafilatura.WithOutputFormat("markdown"),
		trafilatura.WithEnableCrawlWebsite(false),
	)

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Web Reader",
		Model:         model,
		Instructions:  "Extract web page content and metadata, then summarize it.",
		Tools:         []toolkit.Tool{extractor},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Summarize https://go.dev/doc/effective_go and tell me its title.", true, true)
}

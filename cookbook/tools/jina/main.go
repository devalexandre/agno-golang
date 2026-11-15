// Example: web search with the Jina Reader toolkit.
//
// JINA_API_KEY is optional (higher rate limits).
//
//	go run ./cookbook/tools/jina
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/jina"
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

	tk := jina.New(jina.WithAll(), jina.WithMaxContentLength(8000))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Jina Reader",
		Model:         model,
		Instructions:  "Read web pages and search the web with Jina, then summarize.",
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Read https://go.dev/blog and summarize the latest posts.", true, true)
}

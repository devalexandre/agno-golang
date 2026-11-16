// Example: answer questions from documentation published via llms.txt.
//
//	go run ./cookbook/tools/llmstxt
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/llmstxt"
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

	docs := llmstxt.New(llmstxt.WithAllowedHosts("docs.agno.com"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Docs Assistant",
		Model:         model,
		Instructions:  "First read the llms.txt index, then read only the pages relevant to the question.",
		Tools:         []toolkit.Tool{docs},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Using https://docs.agno.com/llms.txt, explain how to create an agent with tools.", true, true)
}

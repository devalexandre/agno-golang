// Example: work with Linear issues.
//
// Requires LINEAR_API_KEY.
//
//	go run ./cookbook/tools/linear
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/linear"
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

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Linear Assistant",
		Model:         model,
		Instructions:  "Use the Linear tools to answer questions about teams and issues.",
		Tools:         []toolkit.Tool{linear.New()},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Who am I and which high-priority issues are open?", true, true)
}

// Example: run Python in an E2B sandbox with E2BTools.
//
// Requires E2B_API_KEY. The sandbox is created on the first tool call.
//
//	go run ./cookbook/tools/e2b
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/e2b"
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

	tk := e2b.NewE2BTools(e2b.WithE2BTimeout(600))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Code Interpreter",
		Model:         model,
		Instructions:  "Use the E2B sandbox to execute Python code and report the results.",
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Compute the mean and standard deviation of [3, 5, 7, 11, 13] with Python.", true, true)
}

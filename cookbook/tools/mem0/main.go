// Example: long-term user memory with Mem0Tools.
//
// Requires MEM0_API_KEY (Mem0 Platform).
//
//	go run ./cookbook/tools/mem0
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/mem0"
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

	tk := mem0.NewMem0Tools(mem0.WithMem0UserID("john_billings"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Memory Agent",
		Model:         model,
		Instructions:  "Store important facts about the user with Mem0 and search them before answering.",
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("I live in NYC and I love Go. What do you remember about me?", true, true)
}

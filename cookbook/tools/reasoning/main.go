// Example: step-by-step reasoning with ReasoningTools (think / analyze).
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
//	go run ./cookbook/tools/reasoning
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/reasoning"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	rt := reasoning.NewReasoningTools(reasoning.WithFewShot(""))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Reasoner",
		Model:         model,
		Instructions:  "Think before answering.\n" + rt.Instructions,
		Tools:         []toolkit.Tool{rt},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("A bat and a ball cost $1.10 in total. The bat costs $1.00 more than the ball. How much does the ball cost?", true, true)

	for i, s := range rt.Steps() {
		fmt.Printf("step %d: %s (next=%s, confidence=%.1f)\n", i+1, s.Title, s.NextAction, s.Confidence)
	}
}

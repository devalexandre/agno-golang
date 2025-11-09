package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	// Create Ollama Cloud model
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatalf("OLLAMA_API_KEY not configured")
	}

	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// Create loop detection guardrail (max 5 iterations)
	loopDetectionGuardrail := agent.NewLoopDetectionGuardrail(5)

	// Create agent with loop detection
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "Loop Detection Agent",
		Instructions:    "You are a helpful assistant.",
		InputGuardrails: []agent.Guardrail{loopDetectionGuardrail},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Loop Detection Guardrails Example ===\n")
	fmt.Println("✓ Agent with loop detection created (max 5 iterations)\n")

	// Test: Simulate iterations
	runID := "run123"
	for i := 1; i <= 7; i++ {
		fmt.Printf("Iteration %d: ", i)
		prompt := fmt.Sprintf("Iteration %d", i)
		_, err := ag.Run(prompt)
		if err != nil {
			fmt.Printf("✗ Blocked: %v\n", err)
		} else {
			fmt.Printf("✓ Allowed\n")
		}
	}

	// Reset loop counter
	loopDetectionGuardrail.ResetLoopCounter(runID)
	fmt.Println("\n✓ Loop counter reset for run:", runID)

	fmt.Println("\n✓ Loop Detection Configuration:")
	fmt.Printf("  - Max Iterations: 5\n")
	fmt.Printf("  - Run ID: %s\n", runID)
	fmt.Printf("  - Tracking: Per-run iteration count\n")
	fmt.Printf("  - Reset: Manual via ResetLoopCounter()\n")
}

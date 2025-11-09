package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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

	// Create rate limiting guardrail (3 requests per 10 seconds)
	rateLimitGuardrail := agent.NewRateLimitGuardrail(3, 10*time.Second)

	// Create agent with rate limiting
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "Rate Limited Agent",
		Instructions:    "You are a helpful assistant.",
		InputGuardrails: []agent.Guardrail{rateLimitGuardrail},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Rate Limiting Guardrails Example ===\n")
	fmt.Println("✓ Agent with rate limiting created (3 requests per 10 seconds)\n")

	// Test: Make multiple requests
	for i := 1; i <= 5; i++ {
		fmt.Printf("Request %d: ", i)
		prompt := fmt.Sprintf("Question %d: What is AI?", i)
		_, err := ag.Run(prompt)
		if err != nil {
			fmt.Printf("✗ Blocked: %v\n", err)
		} else {
			fmt.Printf("✓ Allowed\n")
		}
	}

	fmt.Println("\n✓ Rate Limiting Configuration:")
	fmt.Printf("  - Max Requests: 3\n")
	fmt.Printf("  - Time Window: 10 seconds\n")
	fmt.Printf("  - User ID: anonymous (default)\n")
	fmt.Printf("  - Tracking: Per-user with context values\n")
}

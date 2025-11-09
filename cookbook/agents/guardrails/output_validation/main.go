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

	// Create output guardrails
	outputGuardrails := agent.NewDefaultOutputGuardrails()

	// Create agent with output guardrails
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:          ctx,
		Model:            model,
		Name:             "Output Validation Agent",
		Instructions:     "You are a helpful assistant.",
		OutputGuardrails: outputGuardrails,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Output Validation Guardrails Example ===\n")
	fmt.Println("✓ Agent with output guardrails created\n")

	// Test 1: Normal query (should pass)
	fmt.Println("Test 1: Normal query")
	prompt := "What is machine learning?"
	response, err := ag.Run(prompt)
	if err != nil {
		fmt.Printf("  ✗ Error: %v\n", err)
	} else {
		content := response.TextContent
		if len(content) > 100 {
			content = content[:100]
		}
		fmt.Printf("  ✓ Success: %s\n\n", content)
	}

	// Test 2: Safe technical question
	fmt.Println("Test 2: Safe technical question")
	prompt = "Explain how neural networks work"
	response, err = ag.Run(prompt)
	if err != nil {
		fmt.Printf("  ✗ Error: %v\n", err)
	} else {
		content := response.TextContent
		if len(content) > 100 {
			content = content[:100]
		}
		fmt.Printf("  ✓ Success: %s\n\n", content)
	}

	// Test 3: Query about data science
	fmt.Println("Test 3: Query about data science")
	prompt = "What are the best practices for data analysis?"
	response, err = ag.Run(prompt)
	if err != nil {
		fmt.Printf("  ✗ Error: %v\n", err)
	} else {
		content := response.TextContent
		if len(content) > 100 {
			content = content[:100]
		}
		fmt.Printf("  ✓ Success: %s\n\n", content)
	}

	// Display guardrail information
	fmt.Println("✓ Active Guardrails:")
	for _, gr := range outputGuardrails {
		fmt.Printf("  - %s: %s\n", gr.GetName(), gr.GetDescription())
	}
}

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	// Create an Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Agent with Retry and Exponential Backoff Example ===")
	fmt.Println("Demonstrates resilient agent with automatic retry logic\n")

	// Create agent with retry configuration
	agentWithRetry, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "ResilientAssistant",
		Description:  "An AI assistant with retry capabilities",
		Instructions: "You are a helpful AI assistant.",

		// Retry configuration
		DelayBetweenRetries: 2,    // 2 seconds initial delay
		ExponentialBackoff:  true, // Double delay on each retry (2s, 4s, 8s, etc.)

		Debug: true, // Enable debug to see retry attempts
	})
	if err != nil {
		log.Fatal(err)
	}

	// Test 1: Simple question with retry capability
	fmt.Println("--- Test 1: Basic Question (with retry safety net) ---")
	response, err := agentWithRetry.Run(
		"What is Go programming language?",
		agent.WithRetries(3), // Try up to 3 times if failures occur
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	// Test 2: Complex question with metadata
	fmt.Println("\n--- Test 2: Complex Question with Metadata ---")
	response, err = agentWithRetry.Run(
		"Explain goroutines and channels in Go with examples",
		agent.WithRetries(3),
		agent.WithMetadata(map[string]interface{}{
			"request_id": "retry-test-001",
			"priority":   "high",
		}),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	// Example without exponential backoff (linear retry)
	fmt.Println("\n--- Test 3: Linear Retry (no exponential backoff) ---")
	linearRetryAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "LinearRetryAssistant",
		Instructions: "You are a helpful AI assistant.",

		DelayBetweenRetries: 1,     // 1 second delay
		ExponentialBackoff:  false, // Fixed delay (1s, 1s, 1s, etc.)

		Debug: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err = linearRetryAgent.Run(
		"What are the benefits of using Go?",
		agent.WithRetries(2),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	fmt.Println("\n✅ Retry and exponential backoff example completed!")
	fmt.Println("\nKey points:")
	fmt.Println("- ExponentialBackoff=true: Delays double each retry (2s, 4s, 8s...)")
	fmt.Println("- ExponentialBackoff=false: Fixed delay between retries")
	fmt.Println("- Use WithRetries() to specify max retry attempts")
	fmt.Println("- Debug mode shows retry attempts in logs")
}

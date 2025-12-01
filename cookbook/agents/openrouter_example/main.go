// Package main demonstrates how to use the OpenRouter integration with Agno.
// OpenRouter provides access to multiple LLM providers through a unified API.
//
// OpenRouter is fully compatible with the OpenAI API, so this implementation
// uses the existing OpenAI-like client internally.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openrouter"
)

func main() {
	// Check if API key is set
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		log.Fatal("Please set the OPENROUTER_API_KEY environment variable")
	}

	// Example 1: Basic usage with OpenRouter
	fmt.Println("=== Example 1: Basic OpenRouter Usage ===")
	basicExample()

	// Example 2: Using different models
	fmt.Println("\n=== Example 2: Using Different Models ===")
	differentModelsExample()

	// Example 3: Streaming response
	fmt.Println("\n=== Example 3: Streaming Response ===")
	streamingExample()

	// Example 4: Using with Agent
	fmt.Println("\n=== Example 4: Using with Agent ===")
	agentExample()
}

func basicExample() {
	// Create OpenRouter chat instance with GPT-4o-mini
	// OpenRouter uses the OpenAI-like implementation internally
	chat, err := openrouter.NewOpenRouterChat(
		models.WithID("x-ai/grok-4.1-fast:free"),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenRouter chat: %v", err)
	}

	// Create messages
	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: "You are a helpful assistant.",
		},
		{
			Role:    models.TypeUserRole,
			Content: "What is the capital of Brazil?",
		},
	}

	// Invoke the model
	ctx := context.Background()
	response, err := chat.Invoke(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to invoke: %v", err)
	}

	fmt.Printf("Response: %s\n", response.Content)
}

func differentModelsExample() {
	// List of models to try
	modelsToTry := []string{
		"x-ai/grok-4.1-fast:free",
		"x-ai/grok-4.1-fast:free",
		"x-ai/grok-4.1-fast:free",
	}

	for _, modelID := range modelsToTry {
		fmt.Printf("\n--- Using model: %s ---\n", modelID)

		chat, err := openrouter.NewOpenRouterChat(
			models.WithID(modelID),
		)
		if err != nil {
			log.Printf("Failed to create chat for %s: %v", modelID, err)
			continue
		}

		messages := []models.Message{
			{
				Role:    models.TypeUserRole,
				Content: "Say hello in one sentence.",
			},
		}

		ctx := context.Background()
		response, err := chat.Invoke(ctx, messages)
		if err != nil {
			log.Printf("Failed to invoke %s: %v", modelID, err)
			continue
		}

		fmt.Printf("Response from %s: %s\n", modelID, response.Content)
	}
}

func streamingExample() {
	chat, err := openrouter.NewOpenRouterChat(
		models.WithID("x-ai/grok-4.1-fast:free"),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenRouter chat: %v", err)
	}

	messages := []models.Message{
		{
			Role:    models.TypeUserRole,
			Content: "Count from 1 to 5, one number per line.",
		},
	}

	ctx := context.Background()
	fmt.Print("Streaming response: ")

	err = chat.InvokeStream(ctx, messages, models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))

	if err != nil {
		log.Fatalf("Failed to stream: %v", err)
	}
	fmt.Println()
}

func agentExample() {
	// Create OpenRouter chat instance
	chat, err := openrouter.NewOpenRouterChat(
		models.WithID("x-ai/grok-4.1-fast:free"),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenRouter chat: %v", err)
	}

	// Create an agent with the OpenRouter model
	myAgent, err := agent.NewAgent(agent.AgentConfig{
		Model:        chat,
		Name:         "OpenRouter Agent",
		Description:  "An agent powered by OpenRouter",
		Instructions: "You are a helpful assistant that provides concise answers.",
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Run the agent
	response, err := myAgent.Run("What are the main features of Go programming language?")
	if err != nil {
		log.Fatalf("Failed to run agent: %v", err)
	}

	fmt.Printf("Agent Response:\n%s\n", response.TextContent)
}

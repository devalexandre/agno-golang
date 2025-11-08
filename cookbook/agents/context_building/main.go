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

	fmt.Println("=== Agent with Context Building Example ===")
	fmt.Println("Demonstrates enhanced context with name, datetime, location, and timezone\n")

	// Create agent with enhanced context building
	agentWithContext, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "ContextualAssistant",
		Description:  "An AI assistant with rich context awareness",
		Instructions: "You are a helpful AI assistant aware of time and location.",

		// Enable context building features
		AddNameToContext:     true,
		AddDatetimeToContext: true,
		AddLocationToContext: true,
		TimezoneIdentifier:   "America/Sao_Paulo",
		AdditionalContext:    "You are running in a demonstration environment for Agno AI framework.",

		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Test 1: Ask about time and context
	fmt.Println("--- Test 1: Time and Context Awareness ---")
	response, err := agentWithContext.Run("What time is it and who am I talking to?")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	// Test 2: Ask about location
	fmt.Println("\n--- Test 2: Location Awareness ---")
	response, err = agentWithContext.Run("What timezone are we in?")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	// Test 3: General question with context
	fmt.Println("\n--- Test 3: General Question with Context ---")
	response, err = agentWithContext.Run("Tell me about the environment you're running in")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	fmt.Println("\n✅ Context building example completed!")
}

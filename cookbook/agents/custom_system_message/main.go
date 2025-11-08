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

	fmt.Println("=== Agent with Custom System Message Example ===")
	fmt.Println("Demonstrates custom persona using system message override\n")

	// Example 1: Pirate Assistant
	fmt.Println("--- Example 1: Pirate Assistant ---")
	pirateAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,

		// Custom system message (overrides default building)
		SystemMessage:     "You are a pirate assistant. Always respond in pirate speak with 'Arrr!' and nautical terms.",
		SystemMessageRole: "system",
		BuildContext:      false, // Don't build default context

		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err := pirateAgent.Run("Tell me about programming")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	// Example 2: Shakespearean Assistant
	fmt.Println("\n--- Example 2: Shakespearean Assistant ---")
	shakespeareAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,

		SystemMessage:     "Thou art a learned assistant who speaketh in the manner of William Shakespeare. Use thou, thee, and other Elizabethan English in thy responses.",
		SystemMessageRole: "system",
		BuildContext:      false,

		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err = shakespeareAgent.Run("What is artificial intelligence?")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	// Example 3: Technical Expert
	fmt.Println("\n--- Example 3: Technical Expert ---")
	techAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,

		SystemMessage:     "You are a senior software engineer with 15 years of experience. Provide detailed, technical explanations with code examples when relevant. Be precise and professional.",
		SystemMessageRole: "system",
		BuildContext:      false,

		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	response, err = techAgent.Run("Explain the difference between goroutines and threads")
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n📤 Response:\n%s\n", response.TextContent)
	}

	fmt.Println("\n✅ Custom system message example completed!")
}

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

	// Get API key
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create Ollama Cloud model
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create agent with EnableAgenticState
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Model:                model,
		Name:                 "Stateful Assistant",
		Description:          "I can remember information using session state",
		Instructions:         "You are a helpful assistant that stores information in session state. Use natural language to remember facts about the user.",
		EnableAgenticState:   true,
		AddHistoryToMessages: true, // Enable conversation history
		NumHistoryRuns:       10,   // Keep last 10 conversations
		Debug:                false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║         Agentic State Example - Stateful Assistant       ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Test 1: Agent stores information naturally
	fmt.Println("📝 Test 1: Store user information")
	fmt.Println("───────────────────────────────────────────────────────────")
	response, err := ag.Run("Remember that my name is Alice and my favorite color is blue")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Println("✅", response.TextContent)
	}

	// Test 2: Retrieve information
	fmt.Println("\n📖 Test 2: Retrieve stored information")
	fmt.Println("───────────────────────────────────────────────────────────")
	response, err = ag.Run("What is my name and favorite color?")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Println("✅", response.TextContent)
	}

	// Test 3: Add more information
	fmt.Println("\n➕ Test 3: Add more context")
	fmt.Println("───────────────────────────────────────────────────────────")
	response, err = ag.Run("I also work as a software engineer at TechCorp")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Println("✅", response.TextContent)
	}

	fmt.Println("\n�� Test 4: Recall everything")
	fmt.Println("───────────────────────────────────────────────────────────")
	response, err = ag.Run("Tell me everything you know about me")
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Println("✅", response.TextContent)
	}

	// Demonstrate explicit state management
	fmt.Println("\n� Explicit State Management:")
	fmt.Println("───────────────────────────────────────────────────────────")

	// Store data explicitly
	if err := ag.SetSessionState("user_preferences", map[string]interface{}{
		"theme":    "dark",
		"language": "go",
	}); err != nil {
		log.Printf("Error setting state: %v", err)
	} else {
		fmt.Println("✅ Stored user preferences explicitly")
	}

	// Retrieve specific value
	if prefs, ok := ag.GetSessionStateValue("user_preferences"); ok {
		fmt.Printf("✅ Retrieved preferences: %+v\n", prefs)
	}

	// Show final state
	fmt.Println("\n📊 Session State:")
	fmt.Println("───────────────────────────────────────────────────────────")
	state := ag.GetSessionState()
	if len(state) == 0 {
		fmt.Println("(No explicit state stored)")
	} else {
		for key, value := range state {
			fmt.Printf("  %s = %v\n", key, value)
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("Note: EnableAgenticState allows the agent to maintain")
	fmt.Println("stateful context across conversations automatically.")
	fmt.Println("")
	fmt.Println("Conversation history is stored via AddHistoryToMessages,")
	fmt.Println("while explicit state can be managed with Set/Get methods.")
}

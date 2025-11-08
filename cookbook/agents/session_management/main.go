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

	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create an Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Agent with Session Management Example ===")
	fmt.Println("Demonstrates session tracking and user identification\n")

	// Create agent
	sessionAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "SessionAssistant",
		Description:  "An AI assistant with session management",
		Instructions: "You are a helpful AI assistant. Remember context from previous messages in the session.",
		Debug:        false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Simulate a conversation session
	sessionID := "session-12345"
	userID := "user-alice"

	fmt.Printf("Starting session: %s for user: %s\n\n", sessionID, userID)

	// Message 1
	fmt.Println("--- Message 1 ---")
	response, err := sessionAgent.Run(
		"My name is Alice and I'm learning Go programming",
		agent.WithSessionID(sessionID),
		agent.WithUserID(userID),
		agent.WithMetadata(map[string]interface{}{
			"message_number": 1,
		}),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("📤 Response:\n%s\n", response.TextContent)
	}

	// Message 2
	fmt.Println("\n--- Message 2 ---")
	response, err = sessionAgent.Run(
		"What are goroutines?",
		agent.WithSessionID(sessionID),
		agent.WithUserID(userID),
		agent.WithMetadata(map[string]interface{}{
			"message_number": 2,
		}),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("📤 Response:\n%s\n", response.TextContent)
	}

	// Message 3
	fmt.Println("\n--- Message 3 ---")
	response, err = sessionAgent.Run(
		"Can you remind me what my name is?",
		agent.WithSessionID(sessionID),
		agent.WithUserID(userID),
		agent.WithMetadata(map[string]interface{}{
			"message_number": 3,
		}),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("📤 Response:\n%s\n", response.TextContent)
	}

	// Different user in same session (edge case demonstration)
	fmt.Println("\n--- Different User ---")
	response, err = sessionAgent.Run(
		"Hello, I'm Bob. What are we discussing?",
		agent.WithSessionID(sessionID),
		agent.WithUserID("user-bob"),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("📤 Response:\n%s\n", response.TextContent)
	}

	fmt.Println("\n✅ Session management example completed!")
	fmt.Println("\nKey points:")
	fmt.Println("- Use WithSessionID() to track conversations")
	fmt.Println("- Use WithUserID() to identify users")
	fmt.Println("- Session history is maintained across messages")
	fmt.Println("- Metadata can track additional context")
}

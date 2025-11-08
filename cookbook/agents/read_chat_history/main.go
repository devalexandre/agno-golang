package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()

	// Get API key
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	sqldbconfig := sqlite.SqliteStorageConfig{
		DBFile: utils.StringPtr("read_chat_history.db"),
	}
	_ = sqldbconfig
	// Create SQLite storage for chat history
	storage, err := sqlite.NewSqliteStorage(sqldbconfig)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create Ollama Cloud model
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create agent with read_chat_history tool enabled
	sessionID := "chat-history-demo"
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:                   ctx,
		Model:                     model,
		Name:                      "Assistant",
		Description:               "I have access to chat history",
		Instructions:              "You are a helpful assistant with access to chat history. You automatically remember previous conversations through the message history.",
		DB:                        storage, // Python compatible: db parameter
		SessionID:                 sessionID,
		EnableReadChatHistoryTool: true,
		AddHistoryToMessages:      true,
		NumHistoryRuns:            10,
		Debug:                     false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Read Chat History Tool Demo ===\n")
	fmt.Println("This example demonstrates the ReadChatHistory default tool.")
	fmt.Println("The agent can access previous conversations automatically.\n")

	// Conversation 1: Store some information
	fmt.Println("\n--- Conversation 1: Storing Information ---")
	runConversation(ctx, ag, "My name is Alex and I'm a software engineer from Brazil.")

	// Conversation 2: Store more context
	fmt.Println("\n--- Conversation 2: More Context ---")
	runConversation(ctx, ag, "I'm currently working on a Go project called agno-golang, which is an AI agent framework.")

	// Conversation 3: Store preferences
	fmt.Println("\n--- Conversation 3: Preferences ---")
	runConversation(ctx, ag, "I prefer using Ollama Cloud with the kimi-k2 model for my AI projects.")

	// Conversation 4: Ask agent to recall earlier information
	fmt.Println("\n--- Conversation 4: Recalling Information ---")
	runConversation(ctx, ag, "What do you know about me? Tell me everything you remember.")

	// Conversation 5: Search for specific topic
	fmt.Println("\n--- Conversation 5: Asking About Specific Topic ---")
	runConversation(ctx, ag, "What was the name of my project?")

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nThe agent successfully recalled previous conversations using AddHistoryToMessages.")
	fmt.Println("The ReadChatHistory tool provides programmatic access to history when needed.")
}

func runConversation(ctx context.Context, ag *agent.Agent, userMessage string) {
	fmt.Printf("User: %s\n", userMessage)

	// Run agent
	run, err := ag.Run(userMessage)
	if err != nil {
		log.Printf("Agent error: %v", err)
		return
	}

	// Print response
	if run.TextContent != "" {
		fmt.Printf("Assistant: %s\n", run.TextContent)
	}

	fmt.Println()
}

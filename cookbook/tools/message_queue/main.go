package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	// 1. Initialize the model (local Ollama)
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Initialize the Message Queue tool (REAL Redis operations)
	queueTool := tools.NewMessageQueueManagerTool()

	// 3. Create the Message Queue Management Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Message Queue Manager",
		Model:         model,
		Instructions:  "You are a message queue expert. Use the MessageQueueManagerTool methods to manage Redis queues: Push to add messages to FIFO queues, Pop to retrieve messages, Publish to publish to channels, GetQueueLength to check queue size, and Ping to test connection. Queues use Redis lists (RPUSH/BLPOP).",
		Tools:         []toolkit.Tool{queueTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Message Queue Management Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Check Redis server status using redis-cli and INFO command",
		"Store a test key-value pair in Redis: SET orders:123 '{\"status\":\"pending\"}'",
		"Retrieve all keys from Redis database using KEYS * command",
	}

	for _, query := range queries {
		fmt.Printf("📨 Query: %s\n", query)
		response, err := ag.Run(query)
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}
		fmt.Println("📋 Response:")
		fmt.Println(response.TextContent)
		fmt.Println("\n" + string([]byte{45, 45, 45, 45, 45, 45, 45, 45, 45, 45}) + "\n")
	}
}

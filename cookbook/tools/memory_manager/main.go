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

	// 2. Initialize the Context Aware Memory Manager tool
	memoryTool := tools.NewContextAwareMemoryManager()

	// 3. Create the Agent Memory Manager
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Agent Memory Manager",
		Model:         model,
		Instructions:  "You are an agent memory manager. Help users store and retrieve agent memory, preferences, and context. Manage persistent data across conversations using the context-aware memory system.",
		Tools:         []toolkit.Tool{memoryTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Agent Memory Management Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Store user preferences in memory for personalization",
		"Retrieve previously stored user context and preferences",
		"Update memory with new user interaction data",
	}

	for _, query := range queries {
		fmt.Printf("💭 Query: %s\n", query)
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

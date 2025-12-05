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

	// 2. Initialize the Cache tool (REAL Redis operations)
	cacheTool := tools.NewCacheManagerTool()

	// 3. Create the Cache Management Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Cache Manager",
		Model:         model,
		Instructions:  "You are a cache management expert. Use the CacheManagerTool methods to manage Redis cache: Set to store values, Get to retrieve values, Delete to remove keys, GetAll to find keys matching a pattern, and Info to check Redis status. You can specify TTL for expiration.",
		Tools:         []toolkit.Tool{cacheTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Cache Management Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Check Redis info using redis-cli INFO command",
		"Store test cache data with SET testcache 'cached value' EX 600",
		"Retrieve cached data using GET testcache",
	}

	for _, query := range queries {
		fmt.Printf("💾 Query: %s\n", query)
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

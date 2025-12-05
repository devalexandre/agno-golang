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

	// 2. Initialize the API Client tool
	apiTool := tools.NewAPIClientTool()

	// 3. Create the API Integration Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "API Integration Expert",
		Model:         model,
		Instructions:  "You are an API integration expert. Help users make API requests, handle responses, and integrate with external services. Use GET, POST, PUT, DELETE methods appropriately. When calling tools, ensure all parameters use proper JSON types: objects for headers, numbers for timeouts, strings for URLs.",
		Tools:         []toolkit.Tool{apiTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== API Client Integration Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Make a GET request to the public API endpoint",
		"Send a POST request with JSON data to create a new resource",
		"Handle the response and extract the relevant information",
	}

	for _, query := range queries {
		fmt.Printf("🌐 Query: %s\n", query)
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

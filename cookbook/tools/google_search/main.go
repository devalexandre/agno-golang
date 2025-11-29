package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	// 1. Configuration
	apiKey := os.Getenv("GOOGLE_API_KEY")
	cx := os.Getenv("GOOGLE_CX")

	if apiKey == "" || cx == "" {
		fmt.Println("⚠️  GOOGLE_API_KEY and GOOGLE_CX environment variables are required.")
		fmt.Println("Skipping example execution.")
		return
	}

	// 2. Initialize the model
	ollamaKey := os.Getenv("OLLAMA_API_KEY")
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
		models.WithAPIKey(ollamaKey),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 3. Initialize the Google Search tool
	searchTool := tools.NewGoogleSearchTool(apiKey, cx)

	// 4. Create the Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Search Assistant",
		Model:         model,
		Instructions:  "You are a helpful assistant. Use Google Search to find current information.",
		Tools:         []toolkit.Tool{searchTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 5. Run the agent
	fmt.Println("=== Google Search Example ===")

	query := "What are the latest features in Go 1.24?"
	fmt.Printf("\n🔎 Searching: %s\n", query)

	response, err := ag.Run(query)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("\n🤖 Answer:")
	fmt.Println(response.TextContent)
}

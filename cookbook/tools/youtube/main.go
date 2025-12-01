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
	apiKey := os.Getenv("GOOGLE_API_KEY") // YouTube uses the same Google Cloud API Key

	if apiKey == "" {
		fmt.Println("⚠️  GOOGLE_API_KEY environment variable is required.")
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

	// 3. Initialize the YouTube tool
	ytTool := tools.NewYouTubeTool(apiKey)

	// 4. Create the Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Video Assistant",
		Model:         model,
		Instructions:  "You are a video discovery assistant. Search YouTube for videos to help the user.",
		Tools:         []toolkit.Tool{ytTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 5. Run the agent
	fmt.Println("=== YouTube Tool Example ===")

	query := "Find tutorials on how to use Docker for beginners"
	fmt.Printf("\n🎥 Searching: %s\n", query)

	response, err := ag.Run(query)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("\n🤖 Answer:")
	fmt.Println(response.TextContent)
}

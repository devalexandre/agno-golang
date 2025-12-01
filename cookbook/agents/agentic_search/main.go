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

	// 1. Initialize the model
	apiKey := os.Getenv("OLLAMA_API_KEY")
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Initialize the search tool (DuckDuckGo)
	// This tool allows the agent to search the web for real-time information
	ddgTools := tools.NewDuckDuckGoTool()

	// 3. Create the Research Agent
	researcher, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "Web Researcher",
		Model:   model,
		Instructions: `You are a web researcher. 
Your goal is to find information on the requested topic and provide a concise summary.
Always use the search tool to find the latest information.
Cite your sources if possible.`,
		Tools:         []toolkit.Tool{ddgTools},
		ShowToolsCall: true, // Show the search queries
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	topic := "Latest developments in Quantum Computing 2024"
	fmt.Printf("=== Agentic Search Example ===\n")
	fmt.Printf("🔎 Researching topic: %s\n", topic)
	fmt.Println("==============================")

	response, err := researcher.Run(fmt.Sprintf("Search for information about '%s' and summarize the key findings.", topic))
	if err != nil {
		log.Fatalf("Error during research: %v", err)
	}

	fmt.Println("\n📝 Research Summary:")
	fmt.Println(response.TextContent)
}

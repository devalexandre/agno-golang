package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/exa"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {

	// Get API keys from environment
	ollamaAPIKey := os.Getenv("OLLAMA_API_KEY")
	if ollamaAPIKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	exaAPIKey := os.Getenv("EXA_API_KEY")
	if exaAPIKey == "" {
		log.Fatal("EXA_API_KEY environment variable is required")
		exaAPIKey = "21ac6717-047d-49ed-9a4d-2e25a0430271"
	}

	fmt.Println("🔧 Creating ExaTool...")
	exaTool := exa.NewExaTool(exaAPIKey)

	fmt.Println("✅ ExaTool created")
	fmt.Println()

	// Debug: Print available methods
	fmt.Println("📋 Available methods in ExaTool:")
	for methodName := range exaTool.GetMethods() {
		fmt.Printf("  - %s\n", methodName)
	}
	fmt.Println()

	// Create Ollama Cloud model
	fmt.Println("🤖 Creating Ollama Cloud model...")
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("gpt-oss:20b-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(ollamaAPIKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	fmt.Println("✅ Model created")
	fmt.Println()

	// Create agent with ExaTool
	fmt.Println("🤖 Creating agent with ExaTool...")

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Model:         ollamaModel,
		Tools:         []toolkit.Tool{exaTool},
		ShowToolsCall: true,
		Debug:         false,
		Instructions: `You are a helpful research assistant with access to Exa search tools.

Available tools and when to use them:
- SearchExa: Use to search the web for information
- GetContents: Use to get the full content of specific URLs
- FindSimilar: Use to find pages similar to a given URL
- ExaAnswer: Use to get a direct answer to a question using Exa's AI

IMPORTANT: 
1. Choose the RIGHT tool for each request
2. For general searches, use SearchExa
3. For getting content from URLs, use GetContents
4. For finding similar pages, use FindSimilar
5. For direct answers, use ExaAnswer
6. ALWAYS include the tool's result in your response
7. Format results in a clear, readable way`,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("✅ Agent created")
	fmt.Println()

	// Example queries
	queries := []string{
		"Search for recent news about artificial intelligence",
		"What are the latest developments in quantum computing?",
		"Find information about Go programming language best practices",
	}

	fmt.Println("🚀 Running example queries...")
	fmt.Println()
	fmt.Println("=" + string(make([]byte, 78)) + "=")
	fmt.Println()

	for i, query := range queries {
		fmt.Printf("Query %d: %s\n", i+1, query)
		fmt.Println("-" + string(make([]byte, 78)) + "-")

		response, err := ag.Run(query)
		if err != nil {
			log.Printf("❌ Error: %v\n", err)
			fmt.Println()
			continue
		}

		fmt.Printf("🤖 Response:\n%s\n", response.TextContent)
		fmt.Println()
		fmt.Println("=" + string(make([]byte, 78)) + "=")
		fmt.Println()
	}

	fmt.Println("✅ All queries completed successfully!")
	fmt.Println("\n💡 Tip: You can modify this example to test different search operations")
}

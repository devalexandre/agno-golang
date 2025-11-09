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

	// Get API key from environment
	ollamaAPIKey := os.Getenv("OLLAMA_API_KEY")
	if ollamaAPIKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	fmt.Println("🔧 Creating WeatherTool...")
	weatherTool := tools.NewWeatherTool()

	fmt.Println("✅ WeatherTool created")
	fmt.Println()

	// Debug: Print available methods
	fmt.Println("📋 Available methods in WeatherTool:")
	for methodName, method := range weatherTool.GetMethods() {
		fmt.Printf("  - %s: %s\n", methodName, method.Description)
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

	// Create agent with WeatherTool
	fmt.Println("🤖 Creating agent with WeatherTool...")

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Model:         ollamaModel,
		Tools:         []toolkit.Tool{weatherTool},
		ShowToolsCall: true,
		Debug:         false,
		Instructions: `You are a helpful weather assistant with access to weather information tools.

Available tool:
- GetCurrent: Get current weather for a location (city name or coordinates)

IMPORTANT: 
1. When asked about weather, ALWAYS use the GetCurrent tool
2. You can use either location name (e.g., "London") or coordinates (latitude/longitude)
3. ALWAYS include the tool's result in your response
4. Format the weather information in a clear, readable way
5. Include temperature, wind speed, and any other relevant information from the tool result`,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("✅ Agent created")
	fmt.Println()

	// Example queries
	queries := []string{
		"What's the weather like in London?",
		"Tell me the current weather in Tokyo, Japan",
		"How's the weather in New York City?",
		"What's the temperature in Paris?",
		"Is it raining in São Paulo, Brazil?",
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
	fmt.Println("\n💡 Tip: This example validates that the tool calling system is working correctly")
	fmt.Println("💡 If WeatherTool works but DatabaseTool doesn't, the issue is with the LLM model selection")
}

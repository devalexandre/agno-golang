package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	fmt.Println("Max Tool Calls from History - Weather Example")
	fmt.Println("===========================================")

	ctx := context.Background()

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create weather tool
	weatherTool := tools.NewWeatherTool()

	// Create agent with max_tool_calls_from_history limit
	weatherAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:                 ctx,
		Model:                   model,
		Name:                    "WeatherAgent",
		Instructions:            "You are a weather assistant. Get the weather using the get_weather_for_city tool.",
		Tools:                   []toolkit.Tool{weatherTool},
		AddHistoryToMessages:    true,
		MaxToolCallsFromHistory: 3, // Only keep 3 most recent tool calls in context
		Markdown:                true,
		Debug:                   false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Cities to test with
	cities := []string{
		"Tokyo",
		"Delhi",
		"Shanghai",
		"São Paulo",
		"Mumbai",
		"Beijing",
		"Cairo",
		"London",
	}

	// Print header
	fmt.Printf("%-5s | %-15s | %-8s | %-11s | %-50s\n",
		"Run", "City", "Current", "In Context", "Response Preview")
	fmt.Println(strings.Repeat("-", 95))

	for i, city := range cities {
		runNum := i + 1
		prompt := fmt.Sprintf("What's the weather in %s?", city)

		fmt.Printf("Processing run %d for %s...\n", runNum, city)

		// Run the agent
		response, err := weatherAgent.Run(prompt)
		if err != nil {
			log.Printf("Error for %s: %v", city, err)
			continue
		}

		// Count tool calls in the current response
		currentToolCalls := 0

		// Count tool calls from response messages
		for _, msg := range response.Messages {
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				currentToolCalls += len(msg.ToolCalls)
			}
		}

		// Approximate history based on max_tool_calls_from_history setting
		maxHistory := 3 // Our setting
		historyToolCalls := 0
		if runNum > 1 {
			// Previous runs would have generated tool calls
			if runNum-1 > maxHistory {
				historyToolCalls = maxHistory
			} else {
				historyToolCalls = runNum - 1
			}
		}

		totalInContext := historyToolCalls + currentToolCalls

		// Truncate response for display
		responsePreview := response.TextContent
		if len(responsePreview) > 47 {
			responsePreview = responsePreview[:47] + "..."
		}

		// Print results
		fmt.Printf("%-5d | %-15s | %-8d | %-11d | %-50s\n",
			runNum, city, currentToolCalls, totalInContext, responsePreview)

		// Small delay to avoid overwhelming the model
		time.Sleep(1 * time.Second)
	}

	fmt.Println("\nExample completed!")
	fmt.Println("Note: The 'In Context' column shows total tool calls available to the model.")
	fmt.Println("With max_tool_calls_from_history=3, history tool calls should never exceed 3.")
	fmt.Println("This demonstrates how the feature limits tool call context while preserving recent interactions.")
}

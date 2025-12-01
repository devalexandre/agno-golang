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

	// 1. Initialize the model

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Initialize the YFinance tool
	financeTool := tools.NewYFinanceTool()

	// 3. Create the Financial Analyst Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Wall Street Analyst",
		Model:         model,
		Instructions:  "You are a financial analyst. Use the YFinance tool to get stock prices and analyze market trends. Be concise and professional.",
		Tools:         []toolkit.Tool{financeTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Financial Analyst Example ===")

	stocks := []string{"AAPL", "MSFT", "GOOGL"}

	for _, stock := range stocks {
		fmt.Printf("\n📈 Analyzing: %s\n", stock)
		response, err := ag.Run(fmt.Sprintf("What is the current price of %s and how is it performing today?", stock))
		if err != nil {
			log.Printf("Error analyzing %s: %v", stock, err)
			continue
		}
		fmt.Println("\n🤖 Analysis:")
		fmt.Println(response.TextContent)
		fmt.Println("--------------------------------")
	}
}

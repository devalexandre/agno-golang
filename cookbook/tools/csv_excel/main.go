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

	// 2. Initialize the Shell tool
	shellTool := tools.NewShellTool()

	// 3. Create the Data Processing Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Data Processing Specialist",
		Model:         model,
		Instructions:  "You are a data processing specialist. Use the Execute method with shell=true to run CSV/Excel commands (awk, sed, python, pandas, etc). Help users read CSV files, export data to Excel, and analyze data.",
		Tools:         []toolkit.Tool{shellTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== CSV/Excel Data Processing Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Create a test CSV file at /tmp/sample.csv with sample data and headers",
		"Display the CSV file contents and show line count",
		"Use awk or cut to extract specific columns from the CSV file",
	}

	for _, query := range queries {
		fmt.Printf("📊 Query: %s\n", query)
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

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

	// 3. Create the Monitoring & Alerts Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Monitoring & Alerts Manager",
		Model:         model,
		Instructions:  "You are a monitoring and alerting expert. Use the Execute method with shell=true to run monitoring commands (prometheus, grafana, alertmanager, etc). Help users record metrics, create alerts, and monitor system health.",
		Tools:         []toolkit.Tool{shellTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Monitoring & Alerts Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Show current system CPU and memory usage using top command",
		"Display disk space usage for all mounted filesystems",
		"List running processes sorted by memory consumption",
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

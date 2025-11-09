package main

import (
	"context"
	"encoding/json"
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

	fmt.Println("🚀 Slack Tool Demo")
	fmt.Println("=" + string(make([]byte, 50)))

	// Get Slack token from environment
	slackToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackToken == "" {
		log.Fatal("SLACK_BOT_TOKEN environment variable is required")
	}

	// Create Ollama model
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create Slack tool
	slackTool := tools.NewSlackTool(slackToken)

	// Create agent with Slack tool
	agentInstance, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        ollamaModel,
		Tools:        []toolkit.Tool{slackTool},
		Debug:        false,
		Instructions: "You are a helpful assistant that can interact with Slack. You can send messages, list channels, read message history, and more.",
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Demo 1: List Channels
	fmt.Println("\n1️⃣ Demo: List Slack Channels")
	fmt.Println("-" + string(make([]byte, 50)))

	response, err := agentInstance.Run(ctx, "List all channels in the Slack workspace")
	if err != nil {
		log.Printf("Error listing channels: %v", err)
	} else {
		fmt.Printf("Response: %s\n", response.TextContent)
	}

	// Demo 2: Send Message
	fmt.Println("\n2️⃣ Demo: Send Message to Channel")
	fmt.Println("-" + string(make([]byte, 50)))

	response, err = agentInstance.Run(ctx, "Send a message 'Hello from Agno!' to the #general channel")
	if err != nil {
		log.Printf("Error sending message: %v", err)
	} else {
		fmt.Printf("Response: %s\n", response.TextContent)
	}

	// Demo 3: Get Channel History
	fmt.Println("\n3️⃣ Demo: Get Channel History")
	fmt.Println("-" + string(make([]byte, 50)))

	response, err = agentInstance.Run(ctx, "Get the last 5 messages from the #general channel")
	if err != nil {
		log.Printf("Error getting history: %v", err)
	} else {
		fmt.Printf("Response: %s\n", response.TextContent)
	}

	// Demo 4: Search Messages
	fmt.Println("\n4️⃣ Demo: Search Messages")
	fmt.Println("-" + string(make([]byte, 50)))

	response, err = agentInstance.Run(ctx, "Search for messages containing 'meeting' in the workspace")
	if err != nil {
		log.Printf("Error searching messages: %v", err)
	} else {
		fmt.Printf("Response: %s\n", response.TextContent)
	}

	// Demo 5: List Users
	fmt.Println("\n5️⃣ Demo: List Users")
	fmt.Println("-" + string(make([]byte, 50)))

	response, err = agentInstance.Run(ctx, "List all users in the workspace")
	if err != nil {
		log.Printf("Error listing users: %v", err)
	} else {
		fmt.Printf("Response: %s\n", response.TextContent)
	}

	// Demo 6: Direct Tool Call (without agent)
	fmt.Println("\n6️⃣ Demo: Direct Tool Call")
	fmt.Println("-" + string(make([]byte, 50)))

	// List channels directly
	paramsJSON, _ := json.Marshal(map[string]interface{}{
		"limit": 10,
	})
	result, err := slackTool.Execute("list_channels", paramsJSON)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("Channels: %v\n", result)
	}

	fmt.Println("\n✨ Demo completed successfully!")
	fmt.Println("\nNote: Make sure you have set SLACK_BOT_TOKEN environment variable")
	fmt.Println("You can create a Slack app at https://api.slack.com/apps")
}

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

	// 2. Initialize the Wikipedia tool
	wikiTool := tools.NewWikipediaTool()

	// 3. Create the Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Knowledge Assistant",
		Model:         model,
		Instructions:  "You are a knowledgeable assistant. Use Wikipedia to answer questions about history, science, and general knowledge.",
		Tools:         []toolkit.Tool{wikiTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Wikipedia Tool Example ===")

	// Example 1: Historical question
	question := "Who was Ada Lovelace and why is she famous?"
	fmt.Printf("\n❓ Question: %s\n", question)
	response, err := ag.Run(question)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println("\n🤖 Answer:")
	fmt.Println(response.TextContent)

	// Example 2: Scientific concept
	question2 := "Explain the theory of relativity briefly."
	fmt.Printf("\n\n❓ Question: %s\n", question2)
	response, err = ag.Run(question2)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println("\n🤖 Answer:")
	fmt.Println(response.TextContent)
}

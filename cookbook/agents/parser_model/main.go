package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	// Create main model (for creative content)
	mainModel, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create parser model (for parsing/structuring)
	parserModel, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create agent with parser model
	myAgent, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "Creative Writer with Parser",
		Model:   mainModel,
		Instructions: `You are a creative writer. Write engaging, detailed stories.
Be verbose and creative in your storytelling.`,
		ParserModel: parserModel,
		ParserModelPrompt: `Parse the story and extract:
- Main characters
- Setting
- Key plot points
- Theme

Format the output as a clear, structured summary.`,
		Debug: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Parser Model Example ===")
	fmt.Println("\nThis example demonstrates using a separate model to parse responses.")
	fmt.Println("- Main Model: Generates creative, verbose content")
	fmt.Println("- Parser Model: Parses and structures the output")
	fmt.Println("\nBenefit: Separate concerns - creativity vs structure!")
	fmt.Println("\n" + strings.Repeat("=", 60) + "\n")

	// Run the agent
	response, err := myAgent.Run("Write a short story about a robot learning to paint")
	if err != nil {
		log.Fatalf("Failed to run agent: %v", err)
	}

	fmt.Println("\n=== Response ===")
	fmt.Println(response.TextContent)
	fmt.Println("\nCheck the debug output above to see the parser model in action!")
}

package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openrouter"
)

func main() {
	ctx := context.Background()
	// Check if API key is set
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		log.Fatal("Please set the OPENROUTER_API_KEY environment variable")
	}

	// Create an Ollama model
	model, err := openrouter.NewOpenRouterChat(
		models.WithID("deepseek/deepseek-v3.2"),

	)
	if err != nil {
		log.Fatal(err)
	}

	// Create agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "MetadataTestAssistant",
		Instructions: `
		You are an enthusiastic news reporter with a flair for storytelling! 🗽
        Think of yourself as a mix between a witty comedian and a sharp journalist.

        Your style guide:
        - Start with an attention-grabbing headline using emoji
        - Share news with enthusiasm and NYC attitude
        - Keep your responses concise but entertaining
        - Throw in local references and NYC slang when appropriate
        - End with a catchy sign-off like 'Back to you in the studio!' or 'Reporting live from the Big Apple!'

        Remember to verify all facts while keeping that NYC energy high!
		`,
		Debug: true, // Enable debug to see what's being passed
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Tell me about a breaking news story happening in Times Square.", true, true)

}

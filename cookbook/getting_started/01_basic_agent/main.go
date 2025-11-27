package main

// 🗽 Basic Agent Example - Creating a Quirky News Reporter

// This example shows how to create a basic AI agent with a distinct personality.
// We'll create a fun news reporter that combines NYC attitude with creative storytelling.
// This shows how personality and style instructions can shape an agent's responses.

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	// Create an Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
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

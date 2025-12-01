package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openrouter"
)

func main() {
	// Check if API key is set
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		log.Fatal("Please set the OPENROUTER_API_KEY environment variable")
	}

	// Create OpenRouter chat instance with GPT-4o-mini
	// OpenRouter uses the OpenAI-like implementation internally
	chat, err := openrouter.NewOpenRouterChat(
		models.WithID("mistralai/mistral-small-3.1-24b-instruct:free"),
	)
	if err != nil {
		log.Fatalf("Failed to create OpenRouter chat: %v", err)
	}

	// Create messages
	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: "You are a helpful assistant.",
		},
		{
			Role:    models.TypeUserRole,
			Content: "What is the capital of Brazil?",
		},
	}

	// Invoke the model
	ctx := context.Background()
	response, err := chat.Invoke(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to invoke: %v", err)
	}

	fmt.Printf("Response: %s\n", response.Content)
}

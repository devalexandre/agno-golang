package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/together"
)

func main() {
	ctx := context.Background()

	// Option 1: Using API key from environment variable TOGETHER_API_KEY
	// export TOGETHER_API_KEY="your-api-key"
	// model, err := together.NewTogetherChat(
	//     models.WithID(together.ModelLlama318BInstruct),
	// )

	// Option 2: Providing API key explicitly
	apiKey := os.Getenv("TOGETHER_API_KEY")
	if apiKey == "" {
		apiKey = "your-together-api-key" // Replace with your actual key
	}

	model, err := together.NewTogetherChat(
		models.WithID(together.ModelLlama318BInstruct), // Meta Llama 3.1 8B with tool calling
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Together AI model: %v", err)
	}

	// Create a simple message
	messages := []models.Message{
		{
			Role:    "user",
			Content: "What are the key features of Go programming language?",
		},
	}

	// Invoke the model
	response, err := model.Invoke(ctx, messages)
	if err != nil {
		log.Fatalf("Failed to invoke model: %v", err)
	}

	fmt.Printf("Model: %s\n", response.Model)
	fmt.Printf("Response:\n%s\n", response.Content)
}

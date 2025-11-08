package main

import (
	"context"
	"fmt"
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

	fmt.Println("=== Testing Metadata Passing to Model ===\n")

	// Create agent
	testAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "MetadataTestAssistant",
		Instructions: "You are a helpful AI assistant for testing metadata.",
		Debug:        true, // Enable debug to see what's being passed
	})
	if err != nil {
		log.Fatal(err)
	}

	// Test with metadata
	fmt.Println("--- Test: Running with Metadata ---")
	response, err := testAgent.Run(
		"Hello, this is a test",
		agent.WithMetadata(map[string]interface{}{
			"test_id":     "metadata-test-001",
			"environment": "development",
			"user_info": map[string]string{
				"name":  "Test User",
				"email": "test@example.com",
			},
		}),
		agent.WithSessionID("test-session-123"),
		agent.WithUserID("test-user-456"),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n✅ Response received:\n%s\n", response.TextContent)
	}

	fmt.Println("\n--- Test: Metadata with Images ---")
	response, err = testAgent.Run(
		"Describe this image",
		agent.WithImages(models.Image{
			ID:       "img-001",
			URL:      "https://example.com/test.jpg",
			MimeType: "image/jpeg",
		}),
		agent.WithMetadata(map[string]interface{}{
			"request_type": "image_analysis",
			"priority":     "high",
		}),
	)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("\n✅ Response received:\n%s\n", response.TextContent)
	}

	fmt.Println("\n✅ Metadata test completed!")
	fmt.Println("\nNote: Check debug output to verify metadata is being passed to model")
}

package client_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
)

func TestIntegration_OpenAI_Invoke(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test, OPENAI_API_KEY is not set")
	}

	// Create a new OpenAI integration instance without using a mock.
	instance, err := chat.NewOpenAIChat(openai.WithAPIKey(apiKey), openai.WithID("gpt-3.5-turbo"))
	if err != nil {
		t.Fatalf("Failed to create OpenAI instance: %v", err)
	}

	// Prepare the test message.
	msg := models.Message{
		Role:    models.TypeUserRole,
		Content: "What is the capital of France?",
	}

	// Set a timeout context for the API call.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Invoke the API and get the response.
	response, err := instance.Invoke(ctx, []models.Message{msg})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if response.Content == "" {
		t.Fatal("Expected non-empty response content")
	}
	t.Logf("Received response: %s", response.Content)
}

func TestIntegration_OpenAI_InvokeStream(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test, OPENAI_API_KEY is not set")
	}

	// Create a new OpenAI integration instance without using a mock.
	instance, err := chat.NewOpenAIChat(openai.WithAPIKey(apiKey), openai.WithID("gpt-3.5-turbo"))
	if err != nil {
		t.Fatalf("Failed to create OpenAI instance: %v", err)
	}

	// Prepare the test message.
	msg := models.Message{
		Role:    models.TypeUserRole,
		Content: "Tell me a joke.",
	}

	// Set a timeout context for the streaming API call.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Invoke the streaming API.
	msgStream, err := instance.InvokeStream(ctx, []models.Message{msg})
	if err != nil {
		t.Fatalf("InvokeStream failed: %v", err)
	}

	var fullResponse string
	for m := range msgStream {
		fullResponse += m.Content
	}

	if fullResponse == "" {
		t.Fatal("Expected non-empty streaming response")
	}
	t.Logf("Received streaming response: %s", fullResponse)
}

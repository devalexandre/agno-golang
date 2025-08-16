package chat

import (
	"context"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestNewOpenAIChat(t *testing.T) {
	// Skip test if no API key is provided
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}

	// Test creating a new OpenAI chat instance
	chat, err := NewOpenAIChat(
		models.WithAPIKey(apiKey),
		models.WithID("gpt-4-turbo"),
	)
	if err != nil {
		t.Fatalf("Failed to create OpenAI chat: %v", err)
	}

	if chat == nil {
		t.Fatal("OpenAI chat instance is nil")
	}
}

func TestOpenAIChatInvoke(t *testing.T) {
	// Skip test if no API key is provided
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}

	// Create OpenAI chat instance
	chat, err := NewOpenAIChat(
		models.WithAPIKey(apiKey),
		models.WithID("gpt-4-turbo"),
	)
	if err != nil {
		t.Fatalf("Failed to create OpenAI chat: %v", err)
	}

	// Create a simple message
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Hello, say 'test successful' if you can read this.",
	}

	// Test invoke without options
	response, err := chat.Invoke(context.Background(), []models.Message{message})
	if err != nil {
		t.Fatalf("Failed to invoke chat: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.Content == "" {
		t.Fatal("Response content is empty")
	}

	t.Logf("Response: %s", response.Content)
}

func TestOpenAIChatInvokeWithOptions(t *testing.T) {
	// Skip test if no API key is provided
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}

	// Create OpenAI chat instance
	chat, err := NewOpenAIChat(
		models.WithAPIKey(apiKey),
		models.WithID("gpt-4-turbo"),
	)
	if err != nil {
		t.Fatalf("Failed to create OpenAI chat: %v", err)
	}

	// Create a simple message
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Say exactly 'test' and nothing else.",
	}

	// Test invoke with options
	options := []models.Option{
		models.WithMaxTokens(10),
	}

	response, err := chat.Invoke(context.Background(), []models.Message{message}, options...)
	if err != nil {
		t.Fatalf("Failed to invoke chat with options: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.Content == "" {
		t.Fatal("Response content is empty")
	}

	t.Logf("Response with options: %s", response.Content)
}

func TestOpenAIChatModelParameter(t *testing.T) {
	// Skip test if no API key is provided
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping test")
	}

	// Test different model names
	modelNames := []string{
		"gpt-4-turbo",
		"gpt-4o",
		"gpt-4.1",
		"gpt-4.1-2024-12-17",
	}

	for _, modelName := range modelNames {
		t.Run("model_"+modelName, func(t *testing.T) {
			// Create OpenAI chat instance with specific model
			chat, err := NewOpenAIChat(
				models.WithAPIKey(apiKey),
				models.WithID(modelName),
			)
			if err != nil {
				t.Logf("Failed to create OpenAI chat with model %s: %v", modelName, err)
				return
			}

			// Create a simple message
			message := models.Message{
				Role:    models.TypeUserRole,
				Content: "Hello",
			}

			// Test invoke
			response, err := chat.Invoke(context.Background(), []models.Message{message})
			if err != nil {
				t.Logf("Failed to invoke chat with model %s: %v", modelName, err)
				return
			}

			t.Logf("Model %s works! Response: %s", modelName, response.Content)
		})
	}
}

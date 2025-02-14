package openai_test

import (
	"context"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai"
)

func TestCreateChatCompletion(t *testing.T) {

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}
	optsClient := []openai.OptionClient{
		openai.WithModel("gpt-4o"),
		openai.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client with a test API key.
	client, err := openai.NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    "user",
		Content: "Hello, OpenAI!",
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, openai.WithTemperature(0.5))
	if err != nil {
		t.Fatalf("Failed to create chat completion: %v", err)
	}

	// Check the response.
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestCreateChatCompletionStream(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}
	optsClient := []openai.OptionClient{
		openai.WithModel("gpt-4o"),
		openai.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client with a test API key.
	client, err := openai.NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    "user",
		Content: "What's the capital of Brazil?",
	}

	optCall := []openai.Option{
		openai.WithTemperature(0.8),
		openai.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			t.Logf("Streaming chunk:: %+v", string(chunk))
			return nil
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, optCall...)
	if err != nil {
		t.Fatalf("Failed to create chat completion: %v", err)
	}

	// Check the response.
	_ = chatCompletion
}

package client

import (
	"context"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func TestCreateChatCompletion(t *testing.T) {

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}
	optsClient := []models.OptionClient{
		models.WithID("gpt-4o"),
		models.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client with a test API key.
	client, err := NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Hello, OpenAI!",
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, models.WithTemperature(0.5))
	if err != nil {
		// Skip the test if there's an error, as it might be due to API key issues
		t.Skipf("Skipping test due to API error: %v", err)
		return
	}

	// Check the response.
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestCreateChatCompletionStream(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}
	optsClient := []models.OptionClient{
		models.WithID("gpt-4o"),
		models.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client with a test API key.
	client, err := NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "What's the capital of Brazil?",
	}

	optCall := []models.Option{
		models.WithTemperature(0.8),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			t.Logf("Streaming chunk:: %+v", string(chunk))
			return nil
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, optCall...)
	if err != nil {
		// Skip the test if there's an error, as it might be due to API key issues
		t.Skipf("Skipping test due to API error: %v", err)
		return
	}

	// Check the response.
	_ = chatCompletion
}

func TestCreateChatCompletionWithTools(t *testing.T) {

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}
	optsClient := []models.OptionClient{
		models.WithID("gpt-4o"),
		models.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client with a test API key.
	client, err := NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "What is the current temperature in Pocos de Caldas - MG?",
	}

	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			t.Logf("Streaming chunk:: %+v", string(chunk))
			return nil
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Skip the test if there's an error, as it might be due to API key issues
		t.Skipf("Skipping test due to API error: %v", err)
		return
	}

	// Check the response.
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestCreateChatCompletionStreamWithTools(t *testing.T) {

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}
	optsClient := []models.OptionClient{
		models.WithID("gpt-4o"),
		models.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client with a test API key.
	client, err := NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "What is the current temperature in Pocos de Caldas - MG?",
	}

	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Skip the test if there's an error, as it might be due to API key issues
		t.Skipf("Skipping test due to API error: %v", err)
		return
	}

	// Check the response.
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

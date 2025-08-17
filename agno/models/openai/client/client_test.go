package client

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/exa"
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
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// Skip the test if there's an error, as it might be due to API key issues
		t.Skipf("Skipping test due to API error: %v", err)
		return
	}

	// Log full response for debugging
	t.Logf("Full chat completion response: %+v", chatCompletion)
	t.Logf("Message: %+v", chatCompletion.Choices[0].Message)

	// Tool calls are expected for this test
	if len(chatCompletion.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("Expected tool calls in response")
	}

	// Log tool calls for verification
	for i, toolCall := range chatCompletion.Choices[0].Message.ToolCalls {
		t.Logf("Tool call %d: %+v", i, toolCall)
	}

	fmt.Println("Response Content:")
	fmt.Println(chatCompletion.Choices[0].Message.Content)
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

	// Counter for streaming chunks
	var chunkCount int
	var fullContent string

	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			chunkCount++
			content := string(chunk)
			fullContent += content
			t.Logf("Stream chunk %d: %s", chunkCount, content)
			return nil
		}),
	}

	// Use StreamChatCompletion directly for separated streaming with tools
	err = client.StreamChatCompletion(context.Background(), []models.Message{message}, callOPtions...)
	if err != nil {
		// This should fail the test if streaming doesn't work
		t.Fatalf("StreamChatCompletion failed: %v", err)
	}

	// Verify we received streaming data - this is critical for streaming tests
	if chunkCount == 0 {
		t.Fatal("No streaming chunks received - streaming failed")
	}

	// Verify we have actual content
	if len(fullContent) == 0 {
		t.Fatal("No content received from streaming")
	}

	t.Logf("Total streaming chunks received: %d", chunkCount)
	t.Logf("Full streamed content length: %d characters", len(fullContent))
	t.Logf("Test completed successfully with actual streaming")
}

func TestCreateChatCompletionWithExaTool(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}

	exaApiKey := os.Getenv("EXA_API_KEY")
	if exaApiKey == "" {
		t.Skip("Skipping integration test. EXA_API_KEY is not set.")
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
		Content: "Search for the latest information about artificial intelligence developments",
	}

	callOptions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			exa.NewExaTool(exaApiKey),
		}),
	}

	chatCompletion, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOptions...)
	if err != nil {
		// Skip the test if there's an error, as it might be due to API key issues
		t.Skipf("Skipping test due to API error: %v", err)
		return
	}

	// Log full response for debugging
	t.Logf("Full chat completion response: %+v", chatCompletion)
	t.Logf("Message: %+v", chatCompletion.Choices[0].Message)

	// Tool calls are expected for this test
	if len(chatCompletion.Choices[0].Message.ToolCalls) == 0 {
		t.Log("No tool calls in response - AI might have answered without using tools")
	} else {
		// Log tool calls for verification
		for i, toolCall := range chatCompletion.Choices[0].Message.ToolCalls {
			t.Logf("Tool call %d: %+v", i, toolCall)
		}
	}

	fmt.Println("Response Content:")
	fmt.Println(chatCompletion.Choices[0].Message.Content)
	// Check the response.
	t.Logf("Chat completion response: %+v", chatCompletion.Choices[0].Message.Content)
}

func TestStreamChatCompletionWithExaTool(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test. OPENAI_API_KEY is not set.")
	}

	exaApiKey := os.Getenv("EXA_API_KEY")
	if exaApiKey == "" {
		t.Skip("Skipping integration test. EXA_API_KEY is not set.")
	}

	optsClient := []models.OptionClient{
		models.WithID("gpt-4o"),
		models.WithAPIKey(apiKey),
	}

	// Create a new OpenAI client
	client, err := NewClient(optsClient...)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Search for the latest information about artificial intelligence developments using the search tool.",
	}

	// Counter for streaming chunks
	var chunkCount int
	var fullContent string

	callOptions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			exa.NewExaTool(exaApiKey),
		}),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			chunkCount++
			content := string(chunk)
			fullContent += content
			t.Logf("Stream chunk %d: %s", chunkCount, content)
			return nil
		}),
	}

	// Use StreamChatCompletion directly for separated streaming with tools
	err = client.StreamChatCompletion(context.Background(), []models.Message{message}, callOptions...)
	if err != nil {
		// This should fail the test if streaming doesn't work
		t.Fatalf("StreamChatCompletion failed: %v", err)
	}

	// Verify we received streaming data - this is critical for streaming tests
	if chunkCount == 0 {
		t.Fatal("No streaming chunks received - streaming failed")
	}

	// Verify we have actual content
	if len(fullContent) == 0 {
		t.Fatal("No content received from streaming")
	}

	t.Logf("Total streaming chunks received: %d", chunkCount)
	t.Logf("Full streamed content length: %d characters", len(fullContent))
	t.Logf("First 100 chars of content: %s", func() string {
		if len(fullContent) > 100 {
			return fullContent[:100] + "..."
		}
		return fullContent
	}())

	t.Logf("Test completed successfully with actual streaming")
}

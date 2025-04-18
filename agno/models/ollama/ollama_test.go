package ollama

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama/client"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/exa"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// checkOllamaAvailable checks if the Ollama server is running
func checkOllamaAvailable() bool {

	resp, err := client.NewClient("llama3.1:8b", "http://localhost:11434", http.DefaultClient).CreateChatCompletion(context.Background(), []models.Message{})
	if err != nil {
		return false
	}
	return resp != nil
}

func TestNewOllamaChat(t *testing.T) {
	if !checkOllamaAvailable() {
		t.Skip("Ollama server is not running")
	}

	options := []models.OptionClient{
		models.WithID("llama3.1:8b"), // Using a model we know works
	}

	ollamaChat, err := NewOllamaChat(options...)
	if err != nil {
		t.Fatalf("Failed to create OllamaChat: %v", err)
	}

	if ollamaChat == nil {
		t.Fatal("OllamaChat instance is nil")
	}
}

func TestOllamaChat_Invoke(t *testing.T) {
	if !checkOllamaAvailable() {
		t.Skip("Ollama server is not running")
	}

	options := []models.OptionClient{
		models.WithID("llama3.1:8b"), // Using a model we know works
	}

	ollamaChat, err := NewOllamaChat(options...)
	if err != nil {
		t.Fatalf("Failed to create OllamaChat: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Hello, Ollama!",
	}

	res, err := ollamaChat.Invoke(context.Background(), []models.Message{message})
	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if res == nil || res.Content == "" {
		t.Fatal("Expected non-empty response from Ollama")
	}

	t.Logf("Ollama response: %+v", res)
}

// using withStream
func TestOllamaChat_InvokeStream(t *testing.T) {
	if !checkOllamaAvailable() {
		t.Skip("Ollama server is not running")
	}

	options := []models.OptionClient{
		models.WithID("llama3.1:8b"), // Using a model we know works
	}

	ollamaChat, err := NewOllamaChat(options...)
	if err != nil {
		t.Fatalf("Failed to create OllamaChat: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Hello, Ollama!",
	}

	callOptions := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		t.Logf("Streaming chunk:: %s", string(chunk))
		return nil
	})

	err = ollamaChat.InvokeStream(context.Background(), []models.Message{message}, callOptions)
	if err != nil {
		t.Fatalf("InvokeStream failed: %v", err)
	}

}

// TestOllamaChat_InvokeWithDebug tests the integration with debug mode
func TestOllamaChat_InvokeWithDebug(t *testing.T) {

	options := []models.OptionClient{
		models.WithID("llama3.1:8b"), // Using a model we know works with tools
	}

	ollamaChat, err := NewOllamaChat(options...)
	if err != nil {
		t.Fatalf("Failed to create OllamaChat: %v", err)
	}

	// Create system and user messages
	systemMessage := models.Message{
		Role:    models.TypeSystemRole,
		Content: "You are a helpful assistant that provides concise answers.",
	}

	userMessage := models.Message{
		Role: models.TypeUserRole,
		Content: `
			Begin by running 3 distinct searches to gather comprehensive information.
        Analyze and cross-reference sources for accuracy and relevance.
        Structure your report following academic standards but maintain readability.
        Include only verifiable facts with proper citations.
        Create an engaging narrative that guides the reader through complex topics.
        End with actionable takeaways and future implications`,
	}

	// Context with debug flag
	ctx := context.WithValue(context.Background(), models.DebugKey, true)

	exaApiKey := os.Getenv("EXA_API_KEY")
	// Options with tools
	callOptions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			exa.NewExaTool(exaApiKey),
		}),
	}

	res, err := ollamaChat.Invoke(ctx, []models.Message{systemMessage, userMessage}, callOptions...)
	if err != nil {
		t.Fatalf("Invoke with debug failed: %v", err)
	}

	if res == nil || res.Content == "" {
		t.Fatal("Expected non-empty response from Ollama")
	}

	t.Logf("Ollama response with debug: %+v", res)
}

// TestOllamaChat_InvokeWithTools tests the integration with tools
func TestOllamaChat_InvokeWithTools(t *testing.T) {
	if !checkOllamaAvailable() {
		t.Skip("Ollama server is not running")
	}

	options := []models.OptionClient{
		models.WithID("llama3.1:8b"), // Using model recommended for tools
	}

	ollamaChat, err := NewOllamaChat(options...)
	if err != nil {
		t.Fatalf("Failed to create OllamaChat: %v", err)
	}

	// Create system and user messages
	systemMessage := models.Message{
		Role:    models.TypeSystemRole,
		Content: "You are a helpful assistant that can provide weather information.",
	}

	userMessage := models.Message{
		Role:    models.TypeUserRole,
		Content: "What is the current temperature in New York?",
	}

	// Context with flags for debug and showing tool calls
	ctx := context.Background()
	ctx = context.WithValue(ctx, models.DebugKey, true)
	ctx = context.WithValue(ctx, models.ShowToolsCallKey, true)

	exaApiKey := os.Getenv("EXA_API_KEY")
	// Options with weather tool
	callOptions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			exa.NewExaTool(exaApiKey),
		}),
	}

	// Use AInvokeStream to get the complete response after tool execution
	respStream, errChan := ollamaChat.AInvokeStream(ctx, []models.Message{systemMessage, userMessage}, callOptions...)

	// Collect the complete response
	var fullResponse string
	for resp := range respStream {
		t.Logf("Chunk: %s", resp.Content)
		fullResponse += resp.Content
	}

	// Check for errors
	if err, ok := <-errChan; ok && err != nil {
		t.Fatalf("AInvokeStream with tools failed: %v", err)
	}

	if fullResponse == "" {
		t.Fatal("Expected non-empty response from Ollama")
	}

	t.Logf("Ollama full response with tools: %s", fullResponse)
}

// TestOllamaChat_InvokeStreamWithTools tests the integration with tools in streaming mode
func TestOllamaChat_InvokeStreamWithTools(t *testing.T) {
	if !checkOllamaAvailable() {
		t.Skip("Ollama server is not running")
	}

	options := []models.OptionClient{
		models.WithID("llama3.1:8b"), // Using model recommended for tools
	}

	ollamaChat, err := NewOllamaChat(options...)
	if err != nil {
		t.Fatalf("Failed to create OllamaChat: %v", err)
	}

	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "What is the current temperature in Pocos de Caldas - MG?",
	}

	// Context with flag to show tool calls
	ctx := context.WithValue(context.Background(), models.ShowToolsCallKey, true)

	// Options with weather tool and streaming function
	callOptions := []models.Option{
		models.WithTemperature(0.5),
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			t.Logf("Streaming chunk:: %s", string(chunk))
			return nil
		}),
	}

	err = ollamaChat.InvokeStream(ctx, []models.Message{message}, callOptions...)
	if err != nil {
		t.Fatalf("InvokeStream with tools failed: %v", err)
	}

}

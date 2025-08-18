package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/exa"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func TestOllama_CreateChatCompletion(t *testing.T) {
	client := NewClient("llama3.2", "http://localhost:11434", http.DefaultClient)
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Hello, Ollama!",
	}

	resp, err := client.CreateChatCompletion(context.Background(), []models.Message{message})
	if err != nil {
		t.Fatalf("CreateChatCompletion failed: %v", err)
	}

	if resp.Message.Content == "" {
		t.Error("Expected non-empty message content")
	}

	t.Logf("Response: %+v", resp.Message.Content)
}

func TestOllama_CreateChatCompletionWithTool(t *testing.T) {
	client := NewClient("llama3.1:8b", "http://localhost:11434", http.DefaultClient)
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "What's the weather today in Poços de Caldas?",
	}

	callOptions := []models.Option{
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
	}

	resp, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOptions...)
	if err != nil {
		t.Fatalf("CreateChatCompletion failed: %v", err)
	}

	if resp.Message.Content == "" {
		t.Error("Expected non-empty message content")
	}

	t.Logf("Response: %+v", resp.Message.Content)
}

func TestOllama_CreateChatCompletionWithToolExa(t *testing.T) {
	client := NewClient("llama3.1:8b", "http://localhost:11434", http.DefaultClient)
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Search for information about the history of the internet.",
	}

	callOptions := []models.Option{
		models.WithTools([]toolkit.Tool{
			exa.NewExaTool(os.Getenv("EXA_API_KEY")),
		}),
	}

	resp, err := client.CreateChatCompletion(context.Background(), []models.Message{message}, callOptions...)
	if err != nil {
		t.Fatalf("CreateChatCompletion failed: %v", err)
	}

	if resp.Message.Content == "" {
		t.Error("Expected non-empty message content")
	}

	t.Logf("Response: %+v", resp.Message.Content)
}

func TestOllama_StreamChatCompletion(t *testing.T) {
	client := NewClient("llama3.1:8b", "http://localhost:11434", http.DefaultClient)
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "Tell me a short story about a robot learning to paint.",
	}

	// Criar contexto com timeout
	ctx := context.Background()
	var response string
	callOPtions := []models.Option{
		models.WithTemperature(0.5),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			response += string(chunk)
			return nil
		}),
	}
	// Call the streaming method
	err := client.StreamChatCompletion(ctx, []models.Message{message}, callOPtions...)
	if err != nil {
		t.Fatalf("StreamChatCompletion failed: %v", err)
	}

	t.Logf("Response: %+v", response)
}

func TestOllama_StreamChatCompletionWithTool(t *testing.T) {
	client := NewClient("llama3.1:8b", "http://localhost:11434", http.DefaultClient)
	message := models.Message{
		Role:    models.TypeUserRole,
		Content: "What's the weather today in São Paulo?",
	}

	// Configure options with tools

	// Create context with flags for debug and tool display
	ctx := context.WithValue(context.Background(), models.ShowToolsCallKey, true)

	var response string
	callOPtions := []models.Option{
		models.WithTools([]toolkit.Tool{
			tools.NewWeatherTool(),
		}),
		models.WithTemperature(0.5),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Println("Chunk received:", string(chunk))
			response += string(chunk)
			return nil
		}),
	}
	// Call the streaming method with tools
	err := client.StreamChatCompletion(ctx, []models.Message{message}, callOPtions...)
	if err != nil {
		t.Fatalf("StreamChatCompletion with tools failed: %v", err)
	}

	fmt.Println(response)
}

package embedder

import (
	"os"
	"testing"
)

func TestMockEmbedder(t *testing.T) {
	embedder := NewMockEmbedder(384)

	// Basic test
	embedding, err := embedder.GetEmbedding("test text")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embedding) != 384 {
		t.Fatalf("Expected 384 dimensions, got: %d", len(embedding))
	}

	// Teste com embedding fixo
	fixedEmbedding := []float64{0.1, 0.2, 0.3}
	embedder.WithFixedEmbedding(fixedEmbedding)

	result, err := embedder.GetEmbedding("test")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected 3 dimensions, got: %d", len(result))
	}

	for i, val := range result {
		if val != fixedEmbedding[i] {
			t.Fatalf("Expected %f, got %f at index %d", fixedEmbedding[i], val, i)
		}
	}
}

func TestMockEmbedderError(t *testing.T) {
	embedder := NewMockEmbedder(384)
	embedder.WithError("test error")

	_, err := embedder.GetEmbedding("test")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "test error" {
		t.Fatalf("Expected 'test error', got: %v", err)
	}
}

func TestMockEmbedderEmptyText(t *testing.T) {
	embedder := NewMockEmbedder(384)

	_, err := embedder.GetEmbedding("")
	if err != ErrEmptyText {
		t.Fatalf("Expected ErrEmptyText, got: %v", err)
	}
}

func TestMockEmbedderWithUsage(t *testing.T) {
	embedder := NewMockEmbedder(384)

	embedding, usage, err := embedder.GetEmbeddingAndUsage("test text")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embedding) != 384 {
		t.Fatalf("Expected 384 dimensions, got: %d", len(embedding))
	}

	if usage == nil {
		t.Fatal("Expected usage information, got nil")
	}

	if usage["model"] != "mock-embedder" {
		t.Fatalf("Expected model 'mock-embedder', got: %v", usage["model"])
	}

	if usage["dimensions"] != 384 {
		t.Fatalf("Expected 384 dimensions in usage, got: %v", usage["dimensions"])
	}
}

func TestOpenAIEmbedder(t *testing.T) {
	// Only runs if API key is defined
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping OpenAI tests")
	}

	embedder := NewOpenAIEmbedder(
		WithAPIKey(apiKey),
		WithModel("text-embedding-3-small"),
	)

	if embedder.GetID() != "text-embedding-3-small" {
		t.Fatalf("Expected ID 'text-embedding-3-small', got: %s", embedder.GetID())
	}

	if embedder.GetDimensions() != 1536 {
		t.Fatalf("Expected 1536 dimensions, got: %d", embedder.GetDimensions())
	}

	// Basic embedding test
	embedding, err := embedder.GetEmbedding("Hello, world!")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embedding) != embedder.GetDimensions() {
		t.Fatalf("Expected %d dimensions, got: %d", embedder.GetDimensions(), len(embedding))
	}

	// Test with usage information
	embedding2, usage, err := embedder.GetEmbeddingAndUsage("Hello, world!")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(embedding2) != embedder.GetDimensions() {
		t.Fatalf("Expected %d dimensions, got: %d", embedder.GetDimensions(), len(embedding2))
	}

	if usage == nil {
		t.Fatal("Expected usage information, got nil")
	}

	if usage["total_tokens"] == nil {
		t.Fatal("Expected total_tokens in usage")
	}
}

func TestOpenAIEmbedderErrors(t *testing.T) {
	// Teste sem API key
	embedder := NewOpenAIEmbedder()
	embedder.APIKey = ""

	_, err := embedder.GetEmbedding("test")
	if err != ErrAPIKeyMissing {
		t.Fatalf("Expected ErrAPIKeyMissing, got: %v", err)
	}

	// Teste com texto vazio
	embedder.APIKey = "fake-key"
	_, err = embedder.GetEmbedding("")
	if err != ErrEmptyText {
		t.Fatalf("Expected ErrEmptyText, got: %v", err)
	}
}

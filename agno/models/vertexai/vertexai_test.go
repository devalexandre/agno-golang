package vertexai

import "testing"

func TestNewVertexAIRequiresProject(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	if _, err := NewVertexAI(Config{}); err == nil {
		t.Fatal("expected error when project is not set")
	}
}

func TestNewVertexAIConstructs(t *testing.T) {
	m, err := NewVertexAI(Config{Project: "test-proj", Model: "gemini-2.0-flash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.GetID() != "gemini-2.0-flash" {
		t.Fatalf("expected model id, got %q", m.GetID())
	}
}

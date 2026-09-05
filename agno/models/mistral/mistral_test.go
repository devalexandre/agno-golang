package mistral

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestDefaultModel(t *testing.T) {
	m, err := NewMistralChat(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, got)
	}
}

func TestCustomModel(t *testing.T) {
	m, err := NewMistralChat(models.WithAPIKey("test-key"), models.WithID("mistral-small-latest"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "mistral-small-latest" {
		t.Fatalf("expected model %q, got %q", "mistral-small-latest", got)
	}
}

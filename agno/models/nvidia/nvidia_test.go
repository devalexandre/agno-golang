package nvidia

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestDefaultModel(t *testing.T) {
	m, err := NewNvidiaChat(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, got)
	}
}

func TestCustomModel(t *testing.T) {
	m, err := NewNvidiaChat(models.WithAPIKey("test-key"), models.WithID("meta/llama-3.1-8b-instruct"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "meta/llama-3.1-8b-instruct" {
		t.Fatalf("expected model %q, got %q", "meta/llama-3.1-8b-instruct", got)
	}
}

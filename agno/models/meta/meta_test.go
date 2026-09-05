package meta

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestDefaultModel(t *testing.T) {
	m, err := NewMetaChat(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, got)
	}
}

func TestCustomModel(t *testing.T) {
	m, err := NewMetaChat(models.WithAPIKey("test-key"), models.WithID("Llama-4-Scout-17B-16E-Instruct-FP8"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "Llama-4-Scout-17B-16E-Instruct-FP8" {
		t.Fatalf("expected model %q, got %q", "Llama-4-Scout-17B-16E-Instruct-FP8", got)
	}
}

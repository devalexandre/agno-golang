package perplexity

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestDefaultModel(t *testing.T) {
	m, err := NewPerplexityChat(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, got)
	}
}

func TestCustomModel(t *testing.T) {
	m, err := NewPerplexityChat(models.WithAPIKey("test-key"), models.WithID("sonar-pro"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "sonar-pro" {
		t.Fatalf("expected model %q, got %q", "sonar-pro", got)
	}
}

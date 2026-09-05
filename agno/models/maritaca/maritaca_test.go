package maritaca

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestDefaultModel(t *testing.T) {
	m, err := NewMaritacaChat(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != DefaultModel {
		t.Fatalf("expected default model %q, got %q", DefaultModel, got)
	}
}

func TestCustomModel(t *testing.T) {
	m, err := NewMaritacaChat(models.WithAPIKey("test-key"), models.WithID("sabiazinho-3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "sabiazinho-3" {
		t.Fatalf("expected model %q, got %q", "sabiazinho-3", got)
	}
}

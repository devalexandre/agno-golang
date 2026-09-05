package anthropic

import (
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	old := os.Getenv("ANTHROPIC_API_KEY")
	_ = os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Setenv("ANTHROPIC_API_KEY", old)

	if _, err := NewClient(); err == nil {
		t.Fatal("expected error when ANTHROPIC_API_KEY is unset and no APIKey option given")
	}
}

func TestNewAnthropicDefaults(t *testing.T) {
	m, err := NewAnthropic(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != defaultModel {
		t.Fatalf("expected default model %q, got %q", defaultModel, got)
	}
}

func TestNewAnthropicCustomID(t *testing.T) {
	m, err := NewAnthropic(models.WithAPIKey("test-key"), models.WithID("claude-opus-4-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "claude-opus-4-1" {
		t.Fatalf("expected model %q, got %q", "claude-opus-4-1", got)
	}
}

func TestBuildParamsSystemAndMessages(t *testing.T) {
	c, err := NewClient(models.WithAPIKey("test-key"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := []models.Message{
		{Role: models.TypeSystemRole, Content: "you are helpful"},
		{Role: models.TypeUserRole, Content: "hello"},
		{Role: models.TypeAssistantRole, Content: "hi there"},
	}

	params, _ := c.buildParams(msgs, models.DefaultCallOptions())

	if len(params.System) != 1 || params.System[0].Text != "you are helpful" {
		t.Fatalf("system instruction not extracted correctly: %+v", params.System)
	}
	if len(params.Messages) != 2 { // user + assistant (system goes to System)
		t.Fatalf("expected 2 conversation messages, got %d", len(params.Messages))
	}
	if params.MaxTokens != int64(defaultMaxTokens) {
		t.Fatalf("expected default max tokens %d, got %d", defaultMaxTokens, params.MaxTokens)
	}
	if string(params.Model) != defaultModel {
		t.Fatalf("expected model %q, got %q", defaultModel, params.Model)
	}
}

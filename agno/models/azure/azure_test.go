package azure

import (
	"testing"
)

func TestNewAzureChat(t *testing.T) {
	m, err := NewAzureChat(Config{
		APIKey:     "test-key",
		Endpoint:   "https://my-res.openai.azure.com",
		Deployment: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := m.GetID(); got != "gpt-4o" {
		t.Fatalf("expected deployment as ID 'gpt-4o', got %q", got)
	}
}

func TestNewAzureChatDefaultAPIVersion(t *testing.T) {
	// Just ensures construction succeeds with minimal config.
	if _, err := NewAzureChat(Config{APIKey: "k", Endpoint: "https://x.openai.azure.com", Deployment: "d"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

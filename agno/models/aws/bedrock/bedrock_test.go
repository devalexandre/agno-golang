package bedrock

import (
	"context"
	"testing"
)

func TestNewBedrockDefaults(t *testing.T) {
	// Construction loads the AWS config chain; with no creds it still builds the
	// client (calls would fail, but construction must succeed).
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	m, err := NewBedrock(context.Background(), Config{})
	if err != nil {
		t.Fatalf("NewBedrock: %v", err)
	}
	if m.GetID() != DefaultModel {
		t.Fatalf("default model = %q, want %q", m.GetID(), DefaultModel)
	}
}

func TestNewBedrockCustomModel(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	m, err := NewBedrock(context.Background(), Config{Model: "meta.llama3-70b-instruct-v1:0"})
	if err != nil {
		t.Fatalf("NewBedrock: %v", err)
	}
	if m.GetID() != "meta.llama3-70b-instruct-v1:0" {
		t.Fatalf("model = %q", m.GetID())
	}
}

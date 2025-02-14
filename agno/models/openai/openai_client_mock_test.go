package openai

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

// TestOpenAI_Invoke_WithMock_Success tests the Invoke method with a mock client simulating a successful response.
func TestOpenAI_Invoke_WithMock_Success(t *testing.T) {
	expectedContent := "mocked response from client mock"
	clientMock := NewClientMock(SimulateChatCompletionResponse(expectedContent))
	opts := DefaultOptions()
	opts.APIKey = "dummy-api-key"
	opts.Model = "dummy-model"
	instance := &OpenAI{
		client: clientMock,
		opts:   opts,
	}
	messages := []models.Message{
		{Role: models.TypeUserRole, Content: "Test message"},
	}
	msg, err := instance.Invoke(context.Background(), messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected a non-nil message")
	}
	if msg.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, msg.Content)
	}
}

// TestOpenAI_Invoke_WithMock_Error tests the Invoke method with a mock client simulating an error response.
func TestOpenAI_Invoke_WithMock_Error(t *testing.T) {
	errorMessage := "simulated error"
	clientMock := NewClientMock(SimulateChatCompletionError(errorMessage))
	opts := DefaultOptions()
	opts.APIKey = "dummy-api-key"
	opts.Model = "dummy-model"
	instance := &OpenAI{
		client: clientMock,
		opts:   opts,
	}
	messages := []models.Message{
		{Role: models.TypeUserRole, Content: "Test message"},
	}
	msg, err := instance.Invoke(context.Background(), messages)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if err.Error() != errorMessage {
		t.Errorf("expected error message %q, got %q", errorMessage, err.Error())
	}
	if msg != nil {
		t.Errorf("expected nil message on error, got %+v", msg)
	}
}

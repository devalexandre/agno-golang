package openai

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/devalexandre/agno-golang/agno/models"
)

// ClientMock is a mock implementation of ClientInterface for testing purposes.
// It assumes that ClientInterface (including both Do and CreateChatCompletion methods)
// is defined in another file within this package.
type ClientMock struct {
	// DoFunc simulates the behavior of the Do method.
	DoFunc func(ctx context.Context, method, path string, body interface{}, v interface{}) error
}

// Do delegates execution to the DoFunc if it is defined, otherwise returns nil.
func (cm *ClientMock) Do(ctx context.Context, method, path string, body interface{}, v interface{}) error {
	if cm.DoFunc != nil {
		return cm.DoFunc(ctx, method, path, body, v)
	}
	return nil
}

// CreateChatCompletion simulates a chat completion request by invoking Do.
// It uses a dummy HTTP method and path. The DoFunc should simulate the response.
func (cm *ClientMock) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	var resp CompletionResponse
	err := cm.Do(ctx, httpMethodChatCompletion, "/chat/completions", nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (cm *ClientMock) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan ChatCompletionChunk, error) {
	var resp ChatCompletionChunk
	err := cm.Do(ctx, httpMethodChatCompletion, "/chat/completions", nil, &resp)
	if err != nil {
		return nil, err
	}
	ch := make(chan ChatCompletionChunk, 1)
	ch <- resp
	close(ch)
	return ch, nil
}

// httpMethodChatCompletion is a constant representing the HTTP method used for chat completion requests.
const httpMethodChatCompletion = "POST"

// NewClientMock creates and returns a new ClientMock configured with the provided DoFunc.
func NewClientMock(doFunc func(ctx context.Context, method, path string, body interface{}, v interface{}) error) ClientInterface {
	return &ClientMock{
		DoFunc: doFunc,
	}
}

// SimulateChatCompletionResponse returns a function that simulates a successful chat completion response.
// It populates v with a CompletionResponse containing the provided content.
func SimulateChatCompletionResponse(content string) func(ctx context.Context, method, path string, body interface{}, v interface{}) error {
	return func(ctx context.Context, method, path string, body interface{}, v interface{}) error {
		if resp, ok := v.(*CompletionResponse); ok {
			*resp = CompletionResponse{
				ID:      "mock-id",
				Object:  "chat.completion",
				Created: 0,
				Model:   "mock-model",
				Choices: []Choices{
					{
						Index:        0,
						FinishReason: "stop",
						Message: models.MessageResponse{
							Role:    models.TypeAssistantRole,
							Content: content,
						},
					},
				},
			}
			return nil
		}
		// For other types, simulate by marshalling and unmarshalling.
		data, err := json.Marshal(map[string]string{"content": content})
		if err != nil {
			return err
		}
		return json.Unmarshal(data, v)
	}
}

// SimulateChatCompletionError returns a function that simulates an error response from the API.
func SimulateChatCompletionError(errMsg string) func(ctx context.Context, method, path string, body interface{}, v interface{}) error {
	return func(ctx context.Context, method, path string, body interface{}, v interface{}) error {
		return errors.New(errMsg)
	}
}

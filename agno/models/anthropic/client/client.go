package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
)

type AnthropicClient struct {
	apiKey string
	client *http.Client
}

func NewClient(options ...models.OptionClient) (ClientInterface, error) {
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		// Try environment variable
	}

	return &AnthropicClient{
		apiKey: apiKey,
		client: &http.Client{},
	}, nil
}

func (c *AnthropicClient) CreateMessage(ctx context.Context, messages []models.Message, options ...models.Option) (*AnthropicResponse, error) {
	// Dummy implementation for compilation
	return &AnthropicResponse{}, nil
}

func (c *AnthropicClient) StreamMessage(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Dummy implementation for compilation
	return fmt.Errorf("streaming not implemented")
}

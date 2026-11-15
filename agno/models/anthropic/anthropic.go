// Package anthropic integrates Anthropic's Claude models into agno-golang using
// the official SDK (github.com/anthropics/anthropic-sdk-go). It implements
// models.AgnoModelInterface, mirroring the other providers (OpenAI, Gemini).
//
// Usage:
//
//	model, _ := anthropic.NewAnthropic(
//	    models.WithID("claude-sonnet-4-5"),
//	    models.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
//	)
package anthropic

import (
	"context"
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Anthropic is the Claude model implementation of models.AgnoModelInterface.
type Anthropic struct {
	client *Client
	opts   *models.ClientOptions
}

// NewAnthropic creates a new instance of the Anthropic (Claude) integration.
func NewAnthropic(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	cli, err := NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Anthropic client: %w", err)
	}

	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}
	if opts.ID == "" {
		opts.ID = defaultModel
	}

	return &Anthropic{
		client: cli,
		opts:   opts,
	}, nil
}

// GetID returns the model ID.
func (a *Anthropic) GetID() string {
	return a.opts.ID
}

// GetClientOptions returns the client options for this Anthropic model.
func (a *Anthropic) GetClientOptions() *models.ClientOptions {
	return a.opts
}

// Invoke sends a message request and parses the response into a MessageResponse.
func (a *Anthropic) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	if a.opts.MaxTokens != nil {
		options = append([]models.Option{models.WithMaxTokens(*a.opts.MaxTokens)}, options...)
	}

	resp, err := a.client.CreateChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	return &models.MessageResponse{
		Model:     resp.Model,
		Role:      resp.Choices[0].Message.Role,
		Content:   resp.Choices[0].Message.Content,
		ToolCalls: resp.Choices[0].Message.ToolCalls,
	}, nil
}

// AInvoke is the asynchronous version of Invoke.
func (a *Anthropic) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := a.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
			return
		}
		ch <- resp
	}()
	return ch, errChan
}

// InvokeStream streams the response to the configured StreamingFunc.
func (a *Anthropic) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	if a.opts.MaxTokens != nil {
		options = append([]models.Option{models.WithMaxTokens(*a.opts.MaxTokens)}, options...)
	}

	if err := a.client.StreamChatCompletion(ctx, messages, options...); err != nil {
		return fmt.Errorf("failed to start stream: %w", err)
	}
	return nil
}

// AInvokeStream is the asynchronous version of InvokeStream.
func (a *Anthropic) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse)
	errChan := make(chan error)

	optsFunction := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		ch <- &models.MessageResponse{
			Content: string(chunk),
		}
		return nil
	})
	options = append(options, optsFunction)

	go func() {
		defer close(ch)
		defer close(errChan)
		if err := a.InvokeStream(ctx, messages, options...); err != nil {
			errChan <- err
		}
	}()
	return ch, errChan
}

// New is kept for backward compatibility; it is equivalent to NewAnthropic.
func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	return NewAnthropic(options...)
}

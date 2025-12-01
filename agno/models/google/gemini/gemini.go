package gemini

import (
	"context"
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Gemini is the implementation for the Gemini model of the Agno API
type Gemini struct {
	client *Client
	opts   *models.ClientOptions
}

// NewGemini creates a new instance of the Gemini integration.
func NewGemini(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	cli, err := NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}

	return &Gemini{
		client: cli,
		opts:   opts,
	}, nil
}

// GetID returns the model ID.
func (g *Gemini) GetID() string {
	return g.opts.ID
}

// GetClientOptions returns the client options for this Gemini model
func (g *Gemini) GetClientOptions() *models.ClientOptions {
	return g.opts
}

// Invoke sends a chat completion request and parses the response into a MessageResponse.
func (g *Gemini) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	// Apply client-level options (e.g., MaxTokens) if not already set in call options
	if g.opts.MaxTokens != nil {
		// Prepend the client MaxTokens to options so it acts as a default
		// (call options can still override it)
		options = append([]models.Option{models.WithMaxTokens(*g.opts.MaxTokens)}, options...)
	}

	resp, err := g.client.CreateChatCompletion(ctx, messages, options...)
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

// AInvoke is the asynchronous version of Invoke that uses goroutines and returns a channel of pointers.
func (g *Gemini) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := g.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
		}
		ch <- resp
	}()
	return ch, errChan
}

// InvokeStream implements the streaming method for continuous responses.
func (g *Gemini) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Apply client-level options (e.g., MaxTokens) if not already set in call options
	if g.opts.MaxTokens != nil {
		// Prepend the client MaxTokens to options so it acts as a default
		// (call options can still override it)
		options = append([]models.Option{models.WithMaxTokens(*g.opts.MaxTokens)}, options...)
	}

	err := g.client.StreamChatCompletion(ctx, messages, options...)
	if err != nil {

		return fmt.Errorf("failed to start stream: %w", err)
	}

	return nil

}

// AInvokeStream is the asynchronous version of StreamChatCompletion.
func (g *Gemini) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
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
		err := g.InvokeStream(ctx, messages, options...)
		if err != nil {
			errChan <- err
		}

	}()
	return ch, errChan
}

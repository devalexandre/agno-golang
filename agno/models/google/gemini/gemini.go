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
	opts   *ClientOptions
}

// NewGemini creates a new instance of the Gemini integration.
func NewGemini(options ...OptionClient) (models.AgnoModelInterface, error) {
	cli, err := NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	opts := &ClientOptions{}
	for _, option := range options {
		option(opts)
	}

	return &Gemini{
		client: cli,
		opts:   opts,
	}, nil
}

// Invoke sends a chat completion request and parses the response into a MessageResponse.
func (g *Gemini) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {

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
func (g *Gemini) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, error) {
	responseChannel := make(chan *models.MessageResponse)

	stream, err := g.client.StreamChatCompletion(ctx, messages, options...)
	if err != nil {
		close(responseChannel)
		return nil, fmt.Errorf("failed to start stream: %w", err)
	}

	go func() {
		defer close(responseChannel)
		for msg := range stream {
			responseChannel <- &msg
		}
	}()

	return responseChannel, nil

}

// AInvokeStream is the asynchronous version of StreamChatCompletion.
func (g *Gemini) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		stream, err := g.InvokeStream(ctx, messages, options...)
		if err != nil {
			errChan <- err
		}
		for msg := range stream {
			ch <- msg
		}
	}()
	return ch, errChan
}

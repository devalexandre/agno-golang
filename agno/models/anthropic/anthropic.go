package anthropic

import (
	"context"
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/anthropic/client"
)

type Anthropic struct {
	client client.ClientInterface
	opts   *models.ClientOptions
}

func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	cli, err := client.NewClient(options...)
	if err != nil {
		return nil, err
	}

	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.ID == "" {
		opts.ID = "claude-3-5-sonnet-20240620"
	}

	return &Anthropic{
		client: cli,
		opts:   opts,
	}, nil
}

func (a *Anthropic) GetID() string {
	return a.opts.ID
}

func (a *Anthropic) GetClientOptions() *models.ClientOptions {
	return a.opts
}

func (a *Anthropic) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	if a.opts.MaxTokens != nil {
		options = append([]models.Option{models.WithMaxTokens(*a.opts.MaxTokens)}, options...)
	}

	resp, err := a.client.CreateMessage(ctx, messages, options...)
	if err != nil {
		return nil, err
	}

	if len(resp.Content) == 0 {
		return nil, errors.New("no content in response")
	}

	// For now, we only support text content
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &models.MessageResponse{
		Role:    string(resp.Role),
		Content: content,
		Model:   resp.Model,
	}, nil
}

func (a *Anthropic) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := a.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
		} else {
			ch <- resp
		}
	}()
	return ch, errChan
}

func (a *Anthropic) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	return a.client.StreamMessage(ctx, messages, options...)
}

func (a *Anthropic) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	respChan := make(chan *models.MessageResponse)
	errChan := make(chan error, 1)

	// TODO: Implement streaming functionality for Anthropic
	go func() {
		defer close(respChan)
		defer close(errChan)
		errChan <- fmt.Errorf("streaming not implemented for Anthropic")
	}()

	return respChan, errChan
}

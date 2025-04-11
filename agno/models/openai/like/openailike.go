package like

import (
	"context"
	"errors"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/client"
)

// OpenAILike represents the integration with the OpenAILike API.
type OpenAILike struct {
	client client.ClientInterface
	opts   *client.ClientOptions
}

// NewOpenAILike creates a new instance of the integration with the OpenAILike API.
// This function accepts options as functions that modify *ClientOptions.
func NewOpenAILike(options ...client.OptionClient) (models.AgnoModelInterface, error) {
	cli, err := client.NewClient(options...)
	if err != nil {
		return nil, err
	}

	opts := client.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.BaseURL == "" {
		return nil, errors.New("base URL not set")
	}

	if opts.APIKey == "" {
		return nil, errors.New("API key not set")
	}
	if opts.ID == "" {
		return nil, errors.New("model not set")
	}

	return &OpenAILike{
		client: cli,
		opts:   opts,
	}, nil
}

// ChatCompletion performs a chat completion request.
func (o *OpenAILike) ChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*client.ChatCompletionResponse, error) {
	return o.client.CreateChatCompletion(ctx, messages, options...)
}

// Invoke sends a chat completion request and parses the response into a Message.
func (o *OpenAILike) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	resp, err := o.ChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}
	return &models.MessageResponse{
		Role:    resp.Choices[0].Message.Role,
		Content: resp.Choices[0].Message.Content,
	}, nil
}

// AInvoke is the asynchronous version of Invoke. It delegates to Invoke.
func (o *OpenAILike) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := o.Invoke(ctx, messages, options...)
		if err != nil {
			ch <- &models.MessageResponse{}
			errChan <- err
		} else {
			ch <- resp
		}
	}()
	return ch, errChan
}

// InvokeStream sends a streaming chat completion request and converts each chunk into a Message.
func (o *OpenAILike) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, error) {
	chunkStream, err := o.client.StreamChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	respStream := make(chan *models.MessageResponse)
	go func() {
		defer close(respStream)
		for chunk := range chunkStream {
			if len(chunk.Choices) > 0 {
				respStream <- &models.MessageResponse{
					Role:      chunk.Choices[0].Message.Role,
					Content:   chunk.Choices[0].Message.Content,
					ToolCalls: chunk.Choices[0].Message.ToolCalls,
				}
			}
		}
	}()
	return respStream, nil
}

// AInvokeStream is the asynchronous version of InvokeStream. It delegates to InvokeStream.
func (o *OpenAILike) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	respChan := make(chan *models.MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(respChan)
		defer close(errChan)
		resp, err := o.InvokeStream(ctx, messages, options...)
		if err != nil {
			errChan <- err
			return
		}
		for msg := range resp {
			respChan <- msg
		}
	}()
	return respChan, errChan
}

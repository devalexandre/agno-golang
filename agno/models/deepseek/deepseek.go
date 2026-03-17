package deepseek

import (
	"context"
	"errors"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/client"
)

type DeepSeek struct {
	client client.ClientInterface
	opts   *models.ClientOptions
}

func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.ID == "" {
		opts.ID = "deepseek-chat"
	}

	if opts.BaseURL == "" {
		opts.BaseURL = "https://api.deepseek.com"
	}

	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("DEEPSEEK_API_KEY")
	}

	cli, err := client.NewClient(
		models.WithID(opts.ID),
		models.WithBaseURL(opts.BaseURL),
		models.WithAPIKey(opts.APIKey),
	)
	if err != nil {
		return nil, err
	}

	return &DeepSeek{
		client: cli,
		opts:   opts,
	}, nil
}

func (d *DeepSeek) GetID() string {
	return d.opts.ID
}

func (d *DeepSeek) GetClientOptions() *models.ClientOptions {
	return d.opts
}

func (d *DeepSeek) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	if d.opts.MaxTokens != nil {
		options = append([]models.Option{models.WithMaxTokens(*d.opts.MaxTokens)}, options...)
	}

	resp, err := d.client.CreateChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	return &models.MessageResponse{
		Role:             resp.Choices[0].Message.Role,
		Content:          resp.Choices[0].Message.Content,
		Thinking:         resp.Choices[0].Message.Thinking,
		ToolCalls:        resp.Choices[0].Message.ToolCalls,
		ToolResults:      resp.Choices[0].Message.ToolResults,
		ReasoningContent: resp.Choices[0].Message.ReasoningContent,
	}, nil
}

func (d *DeepSeek) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := d.Invoke(ctx, messages, options...)
		if err != nil {
			ch <- &models.MessageResponse{}
			errChan <- err
		} else {
			ch <- resp
		}
	}()
	return ch, errChan
}

func (d *DeepSeek) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	if d.opts.MaxTokens != nil {
		options = append([]models.Option{models.WithMaxTokens(*d.opts.MaxTokens)}, options...)
	}

	return d.client.StreamChatCompletion(ctx, messages, options...)
}

func (d *DeepSeek) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	respChan := make(chan *models.MessageResponse)
	errChan := make(chan error, 1)

	optsFunction := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		respChan <- &models.MessageResponse{
			Content: string(chunk),
		}
		return nil
	})
	options = append(options, optsFunction)

	err := d.InvokeStream(ctx, messages, options...)
	if err != nil {
		errChan <- err
		return nil, errChan
	}

	return respChan, errChan
}

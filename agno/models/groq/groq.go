package groq

import (
	"context"
	"errors"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/client"
)

type Groq struct {
	client client.ClientInterface
	opts   *models.ClientOptions
}

func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.ID == "" {
		opts.ID = "llama3-70b-8192"
	}

	if opts.BaseURL == "" {
		opts.BaseURL = "https://api.groq.com/openai/v1"
	}

	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("GROQ_API_KEY")
	}

	cli, err := client.NewClient(
		models.WithID(opts.ID),
		models.WithBaseURL(opts.BaseURL),
		models.WithAPIKey(opts.APIKey),
	)
	if err != nil {
		return nil, err
	}

	return &Groq{
		client: cli,
		opts:   opts,
	}, nil
}

func (g *Groq) GetID() string {
	return g.opts.ID
}

func (g *Groq) GetClientOptions() *models.ClientOptions {
	return g.opts
}

func (g *Groq) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	if g.opts.MaxTokens != nil {
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
		Role:             resp.Choices[0].Message.Role,
		Content:          resp.Choices[0].Message.Content,
		Thinking:         resp.Choices[0].Message.Thinking,
		ToolCalls:        resp.Choices[0].Message.ToolCalls,
		ToolResults:      resp.Choices[0].Message.ToolResults,
		ReasoningContent: resp.Choices[0].Message.ReasoningContent,
	}, nil
}

func (g *Groq) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := g.Invoke(ctx, messages, options...)
		if err != nil {
			ch <- &models.MessageResponse{}
			errChan <- err
		} else {
			ch <- resp
		}
	}()
	return ch, errChan
}

func (g *Groq) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	if g.opts.MaxTokens != nil {
		options = append([]models.Option{models.WithMaxTokens(*g.opts.MaxTokens)}, options...)
	}

	return g.client.StreamChatCompletion(ctx, messages, options...)
}

func (g *Groq) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	respChan := make(chan *models.MessageResponse)
	errChan := make(chan error, 1)

	optsFunction := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		respChan <- &models.MessageResponse{
			Content: string(chunk),
		}
		return nil
	})
	options = append(options, optsFunction)

	err := g.InvokeStream(ctx, messages, options...)
	if err != nil {
		errChan <- err
		return nil, errChan
	}

	return respChan, errChan
}

package azure

import (
	"context"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/client"
)

type AzureOpenAI struct {
	client client.ClientInterface
	opts   *models.ClientOptions
}

func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("AZURE_OPENAI_API_KEY")
	}

	if opts.BaseURL == "" {
		opts.BaseURL = os.Getenv("AZURE_OPENAI_ENDPOINT")
	}

	// For Azure, we often use the deployment name as the model ID
	if opts.ID == "" {
		opts.ID = os.Getenv("AZURE_OPENAI_DEPLOYMENT_NAME")
	}

	cli, err := client.NewClient(
		models.WithID(opts.ID),
		models.WithBaseURL(opts.BaseURL),
		models.WithAPIKey(opts.APIKey),
	)
	if err != nil {
		return nil, err
	}

	return &AzureOpenAI{
		client: cli,
		opts:   opts,
	}, nil
}

func (a *AzureOpenAI) GetID() string {
	return a.opts.ID
}

func (a *AzureOpenAI) GetClientOptions() *models.ClientOptions {
	return a.opts
}

func (a *AzureOpenAI) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	resp, err := a.client.CreateChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}

	return &models.MessageResponse{
		Role:             resp.Choices[0].Message.Role,
		Content:          resp.Choices[0].Message.Content,
		Model:            a.opts.ID,
		ToolCalls:        resp.Choices[0].Message.ToolCalls,
		ToolResults:      resp.Choices[0].Message.ToolResults,
		ReasoningContent: resp.Choices[0].Message.ReasoningContent,
	}, nil
}

func (a *AzureOpenAI) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
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

func (a *AzureOpenAI) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	return a.client.StreamChatCompletion(ctx, messages, options...)
}

func (a *AzureOpenAI) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	respChan := make(chan *models.MessageResponse)
	errChan := make(chan error, 1)

	optsFunction := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		respChan <- &models.MessageResponse{
			Content: string(chunk),
		}
		return nil
	})
	options = append(options, optsFunction)

	go func() {
		defer close(respChan)
		defer close(errChan)
		if err := a.InvokeStream(ctx, messages, options...); err != nil {
			errChan <- err
		}
	}()

	return respChan, errChan
}

package chat

import (
	"context"
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/client"
)

// OpenAIChat represents the integration with the OpenAIChat API.
type OpenAIChat struct {
	client client.ClientInterface
	opts   *models.ClientOptions
}

// NewOpenAIChat creates a new instance of the integration with the OpenAIChat API.
// This function accepts options as functions that modify *ClientOptions.
func NewOpenAIChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	cli, err := client.NewClient(options...)
	if err != nil {
		return nil, err
	}

	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	return &OpenAIChat{
		client: cli,
		opts:   opts,
	}, nil
}

// GetID returns the model ID.
func (o *OpenAIChat) GetID() string {
	return o.opts.ID
}

// GetClientOptions returns the client options for this OpenAI model
func (o *OpenAIChat) GetClientOptions() *models.ClientOptions {
	return o.opts
}

// ChatCompletion performs a chat completion request.
func (o *OpenAIChat) ChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*client.ChatCompletionResponse, error) {
	return o.client.CreateChatCompletion(ctx, messages, options...)
}

// Invoke sends a chat completion request and parses the response into a Message.
func (o *OpenAIChat) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	// Apply client-level options (e.g., MaxTokens) if not already set in call options
	if o.opts.MaxTokens != nil {
		// Prepend the client MaxTokens to options so it acts as a default
		// (call options can still override it)
		options = append([]models.Option{models.WithMaxTokens(*o.opts.MaxTokens)}, options...)
	}

	resp, err := o.ChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	// Debug: Log response details
	if debugCtx := ctx.Value(models.DebugKey); debugCtx != nil && debugCtx.(bool) {
		fmt.Printf("DEBUG: OpenAI Chat Invoke - Response ID: %s\n", resp.ID)
		fmt.Printf("DEBUG: OpenAI Chat Invoke - Choices count: %d\n", len(resp.Choices))
		fmt.Printf("DEBUG: OpenAI Chat Invoke - First choice role: %s\n", resp.Choices[0].Message.Role)
		fmt.Printf("DEBUG: OpenAI Chat Invoke - First choice content length: %d\n", len(resp.Choices[0].Message.Content))
		fmt.Printf("DEBUG: OpenAI Chat Invoke - First choice content: %.200s...\n", resp.Choices[0].Message.Content)
		fmt.Printf("DEBUG: OpenAI Chat Invoke - First choice finish reason: %s\n", resp.Choices[0].FinishReason)
		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			fmt.Printf("DEBUG: OpenAI Chat Invoke - Tool calls count: %d\n", len(resp.Choices[0].Message.ToolCalls))
		}
	}

	return &models.MessageResponse{
		Role:    resp.Choices[0].Message.Role,
		Content: resp.Choices[0].Message.Content,
	}, nil
}

// AInvoke is the asynchronous version of Invoke. It delegates to Invoke.
func (o *OpenAIChat) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
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
func (o *OpenAIChat) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Apply client-level options (e.g., MaxTokens) if not already set in call options
	if o.opts.MaxTokens != nil {
		// Prepend the client MaxTokens to options so it acts as a default
		// (call options can still override it)
		options = append([]models.Option{models.WithMaxTokens(*o.opts.MaxTokens)}, options...)
	}

	err := o.client.StreamChatCompletion(ctx, messages, options...)
	if err != nil {
		return err
	}
	return nil
}

// AInvokeStream is the asynchronous version of InvokeStream. It delegates to InvokeStream.
func (o *OpenAIChat) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	respChan := make(chan *models.MessageResponse)
	errChan := make(chan error, 1)

	optsFunction := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		respChan <- &models.MessageResponse{
			Content: string(chunk),
		}
		return nil
	})
	options = append(options, optsFunction)

	err := o.InvokeStream(ctx, messages, options...)

	if err != nil {
		errChan <- err
		return nil, errChan
	}

	return respChan, errChan
}

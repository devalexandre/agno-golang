package ollama

import (
	"context"
	"fmt"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama/client"
	"github.com/devalexandre/agno-golang/agno/tools"
)

// OllamaChat represents the integration with the OllamaChat API.
type OllamaChat struct {
	id      string
	baseURL string
	client  *client.Client
	opts    *models.ClientOptions
}

// authTransport wraps an http.RoundTripper to add Authorization header
type authTransport struct {
	transport http.RoundTripper
	apiKey    string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}
	return t.transport.RoundTrip(req)
}

// NewOllamaChat creates a new instance of the integration with the OllamaChat API.
func NewOllamaChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := models.DefaultOptions()
	for _, opt := range options {
		opt(opts)
	}
	if opts.ID == "" {
		opts.ID = "llama3.1:8b"
	}
	if opts.BaseURL == "" {
		opts.BaseURL = "http://localhost:11434"
	}

	// Create HTTP client with custom transport for authorization
	httpClient := http.DefaultClient
	if opts.APIKey != "" {
		httpClient = &http.Client{
			Transport: &authTransport{
				transport: http.DefaultTransport,
				apiKey:    opts.APIKey,
			},
		}
	}

	cli := client.NewClient(opts.ID, opts.BaseURL, httpClient)
	return &OllamaChat{
		id:      opts.ID,
		baseURL: opts.BaseURL,
		client:  cli,
		opts:    opts,
	}, nil
}
func (o *OllamaChat) GetID() string {
	return o.id
}

// GetClientOptions returns the client options for this Ollama model
func (o *OllamaChat) GetClientOptions() *models.ClientOptions {
	return o.opts
}

// Invoke executes a synchronous call to the Ollama model
func (o *OllamaChat) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	resp, err := o.client.CreateChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}

	// Check if resp is nil
	if resp == nil {
		return nil, fmt.Errorf("received nil response from ollama client")
	}

	var toolCalls []tools.ToolCall
	if resp.Message.ToolCalls != nil {
		for _, tc := range resp.Message.ToolCalls {
			toolCalls = append(toolCalls, tools.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: tools.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: string(tc.Function.Arguments),
				},
			})
		}
	}

	return &models.MessageResponse{
		Model:     o.id,
		Role:      resp.Message.Role,
		Content:   resp.Message.Content,
		Thinking:  resp.Message.Thinking,
		ToolCalls: toolCalls,
	}, nil
}

// AInvoke is the asynchronous version of Invoke
func (o *OllamaChat) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := o.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
			return
		}
		ch <- resp
	}()
	return ch, errChan
}

// InvokeStream executes a streaming call to the Ollama model
func (o *OllamaChat) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	err := o.client.StreamChatCompletion(ctx, messages, options...)
	if err != nil {
		return err
	}
	return nil
}

// AInvokeStream is the asynchronous version of InvokeStream
func (o *OllamaChat) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
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

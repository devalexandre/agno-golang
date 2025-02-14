package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/devalexandre/agno-golang/agno/models"
)

// OpenAIInterface replicates the base interface from Agno (Python version).

// OpenAI represents the integration with the OpenAI API.
type OpenAI struct {
	client ClientInterface
	opts   *ClientOptions
}

// NewOpenAI creates a new instance of the integration with the OpenAI API.
// This function accepts options as functions that modify *ClientOptions.
func NewOpenAI(options ...OptionClient) (models.OpenAIInterface, error) {
	cli, err := NewClient(options...)
	if err != nil {
		return nil, err
	}

	opts := DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	return &OpenAI{
		client: cli,
		opts:   opts,
	}, nil
}

// ChatCompletion performs a chat completion request.
func (o *OpenAI) ChatCompletion(ctx context.Context, messages []models.Message, options ...Option) (*ChatCompletionResponse, error) {
	return o.client.CreateChatCompletion(ctx, messages, options...)
}

// Invoke sends a chat completion request and parses the response into a Message.
func (o *OpenAI) Invoke(ctx context.Context, messages []models.Message) (*models.Message, error) {
	resp, err := o.ChatCompletion(ctx, messages)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}
	return &models.Message{
		Role:    resp.Choices[0].Message.Role,
		Content: resp.Choices[0].Message.Content,
	}, nil
}

// AInvoke is the asynchronous version of Invoke. It delegates to Invoke.
func (o *OpenAI) AInvoke(ctx context.Context, messages []models.Message) (*models.Message, error) {
	return o.Invoke(ctx, messages)
}

// StreamChatCompletion performs a streaming chat completion request.
func (o *OpenAI) StreamChatCompletion(ctx context.Context, messages []models.Message) (<-chan ChatCompletionChunk, error) {
	req := ChatCompletionRequest{
		Model:            o.opts.Model,
		Messages:         messages,
		Temperature:      o.opts.Temperature,
		MaxTokens:        o.opts.MaxTokens,
		TopP:             o.opts.TopP,
		FrequencyPenalty: o.opts.FrequencyPenalty,
		PresencePenalty:  o.opts.PresencePenalty,
	}
	clientReal, ok := o.client.(*Client)
	if !ok {
		return nil, errors.New("client does not support streaming")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, clientReal.baseURL+"/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.opts.APIKey))
	httpReq.Header.Set("Content-Type", "application/json")
	chunks := make(chan ChatCompletionChunk)
	go func() {
		defer close(chunks)
		resp, err := clientReal.client.Do(httpReq)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk ChatCompletionChunk
			if err := decoder.Decode(&chunk); err == io.EOF {
				break
			} else if err != nil {
				break
			}
			chunks <- chunk
		}
	}()
	return chunks, nil
}

// InvokeStream sends a streaming chat completion request and converts each chunk into a Message.
func (o *OpenAI) InvokeStream(ctx context.Context, messages []models.Message) (<-chan models.Message, error) {
	chunkStream, err := o.StreamChatCompletion(ctx, messages)
	if err != nil {
		return nil, err
	}
	respStream := make(chan models.Message)
	go func() {
		defer close(respStream)
		for chunk := range chunkStream {
			if len(chunk.Choices) > 0 {
				respStream <- models.Message{
					Role:    chunk.Choices[0].Message.Role,
					Content: chunk.Choices[0].Message.Content,
				}
			}
		}
	}()
	return respStream, nil
}

// AInvokeStream is the asynchronous version of InvokeStream. It delegates to InvokeStream.
func (o *OpenAI) AInvokeStream(ctx context.Context, messages []models.Message) (<-chan models.Message, error) {
	return o.InvokeStream(ctx, messages)
}

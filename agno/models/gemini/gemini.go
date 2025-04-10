package gemini

import (
	"context"
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Gemini é a implementação para o modelo Gemini da API Agno
type Gemini struct {
	client *Client
	opts   *ClientOptions
}

// NewGemini cria uma nova instância da integração com Gemini.
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

// Invoke envia uma solicitação de completamento de chat e analisa a resposta para um MessageResponse.
func (g *Gemini) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {

	resp, err := g.client.CreateChatCompletion(ctx, messages, options...)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}
	return &models.MessageResponse{
		Role:      resp.Choices[0].Message.Role,
		Content:   resp.Choices[0].Message.Content,
		ToolCalls: resp.Choices[0].Message.ToolCalls,
	}, nil
}

// AInvoke é a versão assíncrona de Invoke que usa goroutines e retorna um canal de ponteiros.
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

// InvokeStream implementa o método de streaming para respostas contínuas.
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

// AInvokeStream é a versão assíncrona de StreamChatCompletion.
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

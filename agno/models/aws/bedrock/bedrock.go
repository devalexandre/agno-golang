// Package bedrock integrates AWS Bedrock models into agno-golang via the unified
// Converse API (github.com/aws/aws-sdk-go-v2/service/bedrockruntime). It
// implements models.AgnoModelInterface.
//
// Authentication uses the standard AWS credential chain (env vars, shared
// config, IAM role) via config.LoadDefaultConfig.
//
// Note: this first version implements text generation, system prompts and
// streaming. Tool calling via the Converse toolConfig is a planned follow-up.
package bedrock

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	brtypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/devalexandre/agno-golang/agno/models"
)

// DefaultModel is used when no model ID is provided.
const DefaultModel = "anthropic.claude-3-5-sonnet-20241022-v2:0"

// Bedrock implements models.AgnoModelInterface using the Bedrock Converse API.
type Bedrock struct {
	client *bedrockruntime.Client
	model  string
	maxTok int32
}

// Config holds Bedrock connection parameters.
type Config struct {
	Region    string // AWS region (e.g. us-east-1); falls back to the AWS config chain
	Model     string // model ID (defaults to Claude 3.5 Sonnet)
	MaxTokens int    // default max tokens (defaults to 4096)
}

// NewBedrock creates a new Bedrock model using the default AWS credential chain.
func NewBedrock(ctx context.Context, cfg Config) (models.AgnoModelInterface, error) {
	opts := []func(*awsconfig.LoadOptions) error{}
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("bedrock: load AWS config: %w", err)
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	maxTok := int32(4096)
	if cfg.MaxTokens > 0 {
		maxTok = int32(cfg.MaxTokens)
	}
	return &Bedrock{
		client: bedrockruntime.NewFromConfig(awsCfg),
		model:  model,
		maxTok: maxTok,
	}, nil
}

// GetID returns the model ID.
func (b *Bedrock) GetID() string { return b.model }

func (b *Bedrock) buildInput(messages []models.Message, callOpts *models.CallOptions) *bedrockruntime.ConverseInput {
	var system []brtypes.SystemContentBlock
	var msgs []brtypes.Message

	for _, m := range messages {
		switch m.Role {
		case models.TypeSystemRole:
			system = append(system, &brtypes.SystemContentBlockMemberText{Value: m.Content})
		case models.TypeAssistantRole:
			if m.Content != "" {
				msgs = append(msgs, brtypes.Message{
					Role:    brtypes.ConversationRoleAssistant,
					Content: []brtypes.ContentBlock{&brtypes.ContentBlockMemberText{Value: m.Content}},
				})
			}
		default:
			if m.Content != "" {
				msgs = append(msgs, brtypes.Message{
					Role:    brtypes.ConversationRoleUser,
					Content: []brtypes.ContentBlock{&brtypes.ContentBlockMemberText{Value: m.Content}},
				})
			}
		}
	}

	maxTok := b.maxTok
	if callOpts.MaxTokens != nil {
		maxTok = int32(*callOpts.MaxTokens)
	}
	inf := &brtypes.InferenceConfiguration{MaxTokens: aws.Int32(maxTok)}
	if callOpts.Temperature != nil {
		inf.Temperature = aws.Float32(*callOpts.Temperature)
	}
	if callOpts.TopP != nil {
		inf.TopP = aws.Float32(*callOpts.TopP)
	}

	in := &bedrockruntime.ConverseInput{
		ModelId:         aws.String(b.model),
		Messages:        msgs,
		InferenceConfig: inf,
	}
	if len(system) > 0 {
		in.System = system
	}
	return in
}

// Invoke sends a Converse request and returns the assistant's text.
func (b *Bedrock) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	callOpts := models.DefaultCallOptions()
	for _, o := range options {
		o(callOpts)
	}

	out, err := b.client.Converse(ctx, b.buildInput(messages, callOpts))
	if err != nil {
		return nil, fmt.Errorf("bedrock: converse: %w", err)
	}

	msgOut, ok := out.Output.(*brtypes.ConverseOutputMemberMessage)
	if !ok {
		return nil, errors.New("bedrock: unexpected output type")
	}

	var text strings.Builder
	for _, block := range msgOut.Value.Content {
		if t, ok := block.(*brtypes.ContentBlockMemberText); ok {
			text.WriteString(t.Value)
		}
	}

	return &models.MessageResponse{
		Model:   b.model,
		Role:    string(models.TypeAssistantRole),
		Content: text.String(),
	}, nil
}

// AInvoke is the asynchronous version of Invoke.
func (b *Bedrock) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := b.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
			return
		}
		ch <- resp
	}()
	return ch, errChan
}

// InvokeStream streams the response text to the configured StreamingFunc.
func (b *Bedrock) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	callOpts := models.DefaultCallOptions()
	for _, o := range options {
		o(callOpts)
	}
	in := b.buildInput(messages, callOpts)

	out, err := b.client.ConverseStream(ctx, &bedrockruntime.ConverseStreamInput{
		ModelId:         in.ModelId,
		Messages:        in.Messages,
		System:          in.System,
		InferenceConfig: in.InferenceConfig,
	})
	if err != nil {
		return fmt.Errorf("bedrock: converse stream: %w", err)
	}

	stream := out.GetStream()
	defer stream.Close()

	for event := range stream.Events() {
		if delta, ok := event.(*brtypes.ConverseStreamOutputMemberContentBlockDelta); ok {
			if d, ok := delta.Value.Delta.(*brtypes.ContentBlockDeltaMemberText); ok {
				if callOpts.StreamingFunc != nil {
					if err := callOpts.StreamingFunc(ctx, []byte(d.Value)); err != nil {
						return err
					}
				}
			}
		}
	}
	return stream.Err()
}

// AInvokeStream is the asynchronous version of InvokeStream.
func (b *Bedrock) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse)
	errChan := make(chan error)

	optsFunc := models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		ch <- &models.MessageResponse{Content: string(chunk)}
		return nil
	})
	options = append(options, optsFunc)

	go func() {
		defer close(ch)
		defer close(errChan)
		if err := b.InvokeStream(ctx, messages, options...); err != nil {
			errChan <- err
		}
	}()
	return ch, errChan
}

var _ models.AgnoModelInterface = (*Bedrock)(nil)

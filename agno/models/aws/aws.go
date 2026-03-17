package aws

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/devalexandre/agno-golang/agno/models"
)

type Bedrock struct {
	client *bedrockruntime.Client
	opts   *models.ClientOptions
}

func New(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := models.DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.ID == "" {
		opts.ID = "anthropic.claude-3-5-sonnet-20240620-v1:0"
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	client := bedrockruntime.NewFromConfig(cfg)

	return &Bedrock{
		client: client,
		opts:   opts,
	}, nil
}

func (b *Bedrock) GetID() string {
	return b.opts.ID
}

func (b *Bedrock) GetClientOptions() *models.ClientOptions {
	return b.opts
}

func (b *Bedrock) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	callOpts := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOpts)
	}

	// Prepare Bedrock messages
	var bedrockMessages []types.Message
	var systemPrompts []types.SystemContentBlock

	for _, msg := range messages {
		switch string(msg.Role) {
		case models.TypeSystemRole:
			systemPrompts = append(systemPrompts, &types.SystemContentBlockMemberText{
				Value: msg.Content,
			})
		case models.TypeUserRole:
			bedrockMessages = append(bedrockMessages, types.Message{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: msg.Content,
					},
				},
			})
		case models.TypeAssistantRole:
			bedrockMessages = append(bedrockMessages, types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: msg.Content,
					},
				},
			})
		}
	}

	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(b.opts.ID),
		Messages: bedrockMessages,
		System:   systemPrompts,
		InferenceConfig: &types.InferenceConfiguration{
			Temperature: callOpts.Temperature,
			TopP:        callOpts.TopP,
		},
	}

	if callOpts.MaxTokens != nil {
		input.InferenceConfig.MaxTokens = aws.Int32(int32(*callOpts.MaxTokens))
	}

	output, err := b.client.Converse(ctx, input)
	if err != nil {
		return nil, err
	}

	messageMember, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return nil, fmt.Errorf("unexpected output type from Bedrock")
	}

	var content string
	for _, block := range messageMember.Value.Content {
		if textBlock, ok := block.(*types.ContentBlockMemberText); ok {
			content += textBlock.Value
		}
	}

	return &models.MessageResponse{
		Role:    string(messageMember.Value.Role),
		Content: content,
		Model:   b.opts.ID,
	}, nil
}

func (b *Bedrock) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errChan := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := b.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
		} else {
			ch <- resp
		}
	}()
	return ch, errChan
}

func (b *Bedrock) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	callOpts := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOpts)
	}

	if callOpts.StreamingFunc == nil {
		return fmt.Errorf("streaming function is required for InvokeStream")
	}

	// Prepare Bedrock messages
	var bedrockMessages []types.Message
	var systemPrompts []types.SystemContentBlock

	for _, msg := range messages {
		switch string(msg.Role) {
		case models.TypeSystemRole:
			systemPrompts = append(systemPrompts, &types.SystemContentBlockMemberText{
				Value: msg.Content,
			})
		case models.TypeUserRole:
			bedrockMessages = append(bedrockMessages, types.Message{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: msg.Content,
					},
				},
			})
		case models.TypeAssistantRole:
			bedrockMessages = append(bedrockMessages, types.Message{
				Role: types.ConversationRoleAssistant,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: msg.Content,
					},
				},
			})
		}
	}

	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  aws.String(b.opts.ID),
		Messages: bedrockMessages,
		System:   systemPrompts,
		InferenceConfig: &types.InferenceConfiguration{
			Temperature: callOpts.Temperature,
			TopP:        callOpts.TopP,
		},
	}

	if callOpts.MaxTokens != nil {
		input.InferenceConfig.MaxTokens = aws.Int32(int32(*callOpts.MaxTokens))
	}

	output, err := b.client.ConverseStream(ctx, input)
	if err != nil {
		return err
	}

	for event := range output.GetStream().Events() {
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if delta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
				if err := callOpts.StreamingFunc(ctx, []byte(delta.Value)); err != nil {
					return err
				}
			}
		case *types.ConverseStreamOutputMemberMessageStop:
			// Stream ended
		case *types.ConverseStreamOutputMemberMetadata:
			// Metadata event
		}
	}

	return nil
}

func (b *Bedrock) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
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
		if err := b.InvokeStream(ctx, messages, options...); err != nil {
			errChan <- err
		}
	}()

	return respChan, errChan
}

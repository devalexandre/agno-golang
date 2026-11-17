package anthropic

import (
	"context"
	"fmt"
	"os"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

const (
	// defaultModel is used when no model ID is provided. Override with models.WithID.
	defaultModel = "claude-sonnet-4-5"
	// defaultMaxTokens is required by the Anthropic API; used when MaxTokens is unset.
	defaultMaxTokens = 4096
)

// Client wraps the official Anthropic Go SDK (github.com/anthropics/anthropic-sdk-go).
type Client struct {
	model     string
	apiKey    string
	sdkClient sdk.Client
	options   models.ClientOptions
}

// NewClient creates a new Anthropic client.
func NewClient(options ...models.OptionClient) (*Client, error) {
	opts := models.ClientOptions{}
	for _, option := range options {
		option(&opts)
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
	}

	if opts.ID == "" {
		opts.ID = defaultModel
	}

	reqOpts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if opts.BaseURL != "" {
		reqOpts = append(reqOpts, option.WithBaseURL(opts.BaseURL))
	}

	return &Client{
		model:     opts.ID,
		apiKey:    apiKey,
		sdkClient: sdk.NewClient(reqOpts...),
		options:   opts,
	}, nil
}

// buildParams converts agno messages/options into Anthropic SDK params and
// returns the map of available tools keyed by method name.
func (c *Client) buildParams(messages []models.Message, callOptions *models.CallOptions) (sdk.MessageNewParams, map[string]toolkit.Tool) {
	var system []sdk.TextBlockParam
	var msgs []sdk.MessageParam

	for _, m := range messages {
		switch m.Role {
		case models.TypeSystemRole:
			system = append(system, sdk.TextBlockParam{Text: m.Content})
		case models.TypeAssistantRole:
			if m.Content != "" {
				msgs = append(msgs, sdk.NewAssistantMessage(sdk.NewTextBlock(m.Content)))
			}
		default: // user and tool roles are mapped to user blocks
			if blocks := userBlocks(m); len(blocks) > 0 {
				msgs = append(msgs, sdk.NewUserMessage(blocks...))
			}
		}
	}

	maxTokens := int64(defaultMaxTokens)
	if callOptions.MaxTokens != nil {
		maxTokens = int64(*callOptions.MaxTokens)
	}

	params := sdk.MessageNewParams{
		Model:     sdk.Model(c.model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if len(system) > 0 {
		params.System = system
	}
	if callOptions.Temperature != nil {
		params.Temperature = sdk.Float(float64(*callOptions.Temperature))
	}
	if callOptions.TopP != nil {
		params.TopP = sdk.Float(float64(*callOptions.TopP))
	}
	if stops, ok := callOptions.Stop.([]string); ok && len(stops) > 0 {
		params.StopSequences = stops
	}

	toolDefs, maptools := c.prepareTools(callOptions.ToolCall)
	if len(toolDefs) > 0 {
		params.Tools = toolDefs
	}

	return params, maptools
}

// prepareTools converts agno toolkits into Anthropic tool definitions.
func (c *Client) prepareTools(toolsCall []toolkit.Tool) ([]sdk.ToolUnionParam, map[string]toolkit.Tool) {
	var defs []sdk.ToolUnionParam
	maptools := make(map[string]toolkit.Tool)

	for _, tool := range toolsCall {
		for methodName := range tool.GetMethods() {
			params := tool.GetParameterStruct(methodName)

			schema := sdk.ToolInputSchemaParam{}
			if props, ok := params["properties"]; ok {
				schema.Properties = props
			}
			if req, ok := params["required"].([]string); ok {
				schema.Required = req
			}

			def := sdk.ToolUnionParamOfTool(schema, methodName)
			if def.OfTool != nil {
				if desc := tool.GetDescriptionOfMethod(methodName); desc != "" {
					def.OfTool.Description = sdk.String(desc)
				}
			}

			defs = append(defs, def)
			maptools[methodName] = tool
		}
	}

	return defs, maptools
}

// CreateChatCompletion performs a (non-streaming) message request, resolving any
// tool-use turns by executing the requested tools and continuing the conversation.
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	debugmod := ctx.Value(models.DebugKey)
	showToolsCall := ctx.Value(models.ShowToolsCallKey)

	// Start empty, not from models.DefaultCallOptions(): its temperature and
	// top_p would go out with every request, and Claude models from the 5
	// family on reject sampling parameters ("`temperature` is deprecated for
	// this model"). Only what the caller sets is sent; the API has its own
	// defaults.
	callOptions := &models.CallOptions{}
	for _, opt := range options {
		opt(callOptions)
	}

	params, maptools := c.buildParams(messages, callOptions)

	if debugmod != nil && debugmod.(bool) {
		utils.DebugPanel(fmt.Sprintf("[Anthropic] model=%s messages=%d tools=%d", c.model, len(params.Messages), len(params.Tools)))
	}

	resp, err := c.sdkClient.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}

	// Tool-use loop: keep resolving tool calls until the model returns a final turn.
	for resp.StopReason == sdk.StopReasonToolUse {
		params.Messages = append(params.Messages, resp.ToParam())

		var toolResults []sdk.ContentBlockParamUnion
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}

			tool, ok := maptools[block.Name]
			if !ok {
				return nil, fmt.Errorf("tool %q not found", block.Name)
			}

			if showToolsCall != nil && showToolsCall.(bool) {
				utils.ToolCallPanel(fmt.Sprintf("🚀 Running tool %s with args: %s", block.Name, string(block.Input)))
			}

			result, execErr := tool.Execute(block.Name, block.Input)
			content := fmt.Sprintf("%v", result)
			isErr := false
			if execErr != nil {
				content = execErr.Error()
				isErr = true
			}

			if showToolsCall != nil && showToolsCall.(bool) {
				utils.ToolCallPanel(fmt.Sprintf("✅ Tool %s finished", block.Name))
			}

			toolResults = append(toolResults, sdk.NewToolResultBlock(block.ID, content, isErr))
		}

		params.Messages = append(params.Messages, sdk.NewUserMessage(toolResults...))

		resp, err = c.sdkClient.Messages.New(ctx, params)
		if err != nil {
			return nil, err
		}
	}

	return buildCompletionResponse(c.model, resp), nil
}

// StreamChatCompletion streams the model response to the configured StreamingFunc.
// When tools are configured, the tool-use loop runs non-streaming and the final
// text is emitted to the StreamingFunc.
func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Start empty, not from models.DefaultCallOptions(): its temperature and
	// top_p would go out with every request, and Claude models from the 5
	// family on reject sampling parameters ("`temperature` is deprecated for
	// this model"). Only what the caller sets is sent; the API has its own
	// defaults.
	callOptions := &models.CallOptions{}
	for _, opt := range options {
		opt(callOptions)
	}

	// Tool calls require the resolve loop; fall back to non-streaming then emit result.
	if len(callOptions.ToolCall) > 0 {
		resp, err := c.CreateChatCompletion(ctx, messages, options...)
		if err != nil {
			return err
		}
		if callOptions.StreamingFunc != nil && len(resp.Choices) > 0 {
			return callOptions.StreamingFunc(ctx, []byte(resp.Choices[0].Message.Content))
		}
		return nil
	}

	params, _ := c.buildParams(messages, callOptions)

	stream := c.sdkClient.Messages.NewStreaming(ctx, params)
	for stream.Next() {
		event := stream.Current()
		if event.Delta.Text != "" && callOptions.StreamingFunc != nil {
			if err := callOptions.StreamingFunc(ctx, []byte(event.Delta.Text)); err != nil {
				return err
			}
		}
	}
	return stream.Err()
}

// buildCompletionResponse normalizes the SDK Message into a CompletionResponse.
func buildCompletionResponse(model string, resp *sdk.Message) *CompletionResponse {
	var text string
	for _, block := range resp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return &CompletionResponse{
		ID:     resp.ID,
		Object: "chat.completion",
		Model:  model,
		Choices: []Choices{{
			Index: 0,
			Message: models.MessageResponse{
				Model:   model,
				Role:    string(models.TypeAssistantRole),
				Content: text,
			},
			FinishReason: string(resp.StopReason),
		}},
		Usage: Usage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
}

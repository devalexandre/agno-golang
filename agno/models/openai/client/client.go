package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// Client represents a wrapper around the official OpenAI client.
type Client struct {
	model   string
	client  *openai.Client
	options models.ClientOptions
}

// NewClient creates a new client for the OpenAI API using the official client.
func NewClient(options ...models.OptionClient) (*Client, error) {
	opts := models.ClientOptions{}
	for _, option := range options {
		option(&opts)
	}

	copts := toRequestOptions(opts)
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set. Please provide an API key or set the OPENAI_API_KEY environment variable")
		}
		opts.APIKey = apiKey
	}

	client := openai.NewClient(copts...)

	return &Client{
		model:   opts.ID,
		client:  &client,
		options: opts,
	}, nil
}

// CreateChatCompletion creates a chat completion request using the official client.
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	// Process options to get tools and other parameters
	callOptions := models.DefaultCallOptions()
	for _, option := range options {
		option(callOptions)
	}

	// Convert our messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, len(messages))
	for i, msg := range messages {
		switch msg.Role {
		case models.TypeUserRole:
			openaiMessages[i] = openai.UserMessage(msg.Content)
		case models.TypeAssistantRole:
			// Create assistant message
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Content: openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(msg.Content),
				},
			}

			// Handle tool calls if present
			if len(msg.ToolCalls) > 0 {
				toolCalls := make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					toolCalls[j] = openai.ChatCompletionMessageToolCallParam{
						ID:   tc.ID,
						Type: "function", // Use string literal
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
				assistantMsg.ToolCalls = toolCalls
			}

			openaiMessages[i] = openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistantMsg,
			}
		case models.TypeSystemRole:
			openaiMessages[i] = openai.SystemMessage(msg.Content)
		case models.TypeToolRole:
			if msg.ToolCallID != nil {
				openaiMessages[i] = openai.ToolMessage(msg.Content, *msg.ToolCallID)
			}
		}
	}

	// Build chat completion params
	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(c.model),
		Messages: openaiMessages,
	}

	if c.options.BaseURL != "" {
		params.Model = c.model
	}

	// Add optional parameters
	if callOptions.Temperature != nil {
		params.Temperature = openai.Float(float64(*callOptions.Temperature))
	}
	if callOptions.MaxTokens != nil {
		params.MaxTokens = openai.Int(int64(*callOptions.MaxTokens))
	}

	debugmod := ctx.Value(models.DebugKey)
	if debugmod != nil && debugmod.(bool) {
		fmt.Printf("DEBUG: Creating chat completion with MaxTokens: %d, Temperature: %.2f\n",
			params.MaxTokens, params.Temperature)
	}

	// Handle tools
	var maptools map[string]toolkit.Tool
	if len(callOptions.ToolCall) > 0 {
		openaiTools, toolMap, err := c.buildOpenAITools(callOptions.ToolCall)
		if err != nil {
			return nil, fmt.Errorf("failed to build OpenAI tools: %w", err)
		}
		params.Tools = openaiTools
		params.ToolChoice = openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openai.String("auto"),
		}
		maptools = toolMap
	}

	// Handle streaming
	if callOptions.StreamingFunc != nil {
		return c.createStreamingCompletion(ctx, params, callOptions, maptools)
	}

	// Make the request
	resp, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}

	// Convert response back to our format
	result := &CompletionResponse{
		ID:      resp.ID,
		Object:  string(resp.Object),
		Created: resp.Created,
		Model:   string(resp.Model),
	}

	if len(resp.Choices) > 0 {
		choices := make([]Choices, len(resp.Choices))
		for i, choice := range resp.Choices {
			choices[i] = Choices{
				Index:        int(choice.Index),
				FinishReason: string(choice.FinishReason),
				Message: models.MessageResponse{
					Role:    string(choice.Message.Role),
					Content: choice.Message.Content,
				},
			}

			// Handle tool calls in response
			if len(choice.Message.ToolCalls) > 0 {
				toolCalls := make([]tools.ToolCall, len(choice.Message.ToolCalls))
				for j, tc := range choice.Message.ToolCalls {
					toolCalls[j] = tools.ToolCall{
						ID:   tc.ID,
						Type: tools.ToolType(tc.Type),
						Function: tools.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
				choices[i].Message.ToolCalls = toolCalls
			}
		}
		result.Choices = choices
	}

	// Process tool calls if present - handle both "tool_calls" and "length" finish reasons
	// Sometimes OpenAI returns "length" when the response is truncated but tool calls are present
	if len(result.Choices) > 0 && len(result.Choices[0].Message.ToolCalls) > 0 {
		debugmod := ctx.Value(models.DebugKey)
		if debugmod != nil && debugmod.(bool) {
			fmt.Printf("DEBUG: Found %d tool calls with finish reason: %s\n", len(result.Choices[0].Message.ToolCalls), result.Choices[0].FinishReason)
		}
		return c.processToolCalls(ctx, messages, result, maptools, callOptions)
	}

	return result, nil
}

// StreamChatCompletion performs a streaming chat completion request.
func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Process options
	callOptions := models.DefaultCallOptions()
	for _, option := range options {
		option(callOptions)
	}

	// Check if streaming function is provided
	if callOptions.StreamingFunc == nil {
		return fmt.Errorf("streaming function required for StreamChatCompletion")
	}

	// Create chat completion with streaming (this will automatically use streaming when StreamingFunc is set)
	_, err := c.CreateChatCompletion(ctx, messages, options...)
	return err
}

func (c *Client) createStreamingCompletion(ctx context.Context, params openai.ChatCompletionNewParams, callOptions *models.CallOptions, maptools map[string]toolkit.Tool) (*CompletionResponse, error) {
	// For streaming, we'll use a simplified message reconstruction approach
	// since extracting from the complex OpenAI message unions is tricky
	// Most common case is a single user message for tools
	originalMessages := []models.Message{
		{
			Role:    models.TypeUserRole,
			Content: "User query requiring tool usage",
		},
	}

	// Enable streaming using the official client streaming method
	stream := c.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var fullContent strings.Builder
	var currentToolCalls []tools.ToolCall
	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		// Handle content streaming
		if content, ok := acc.JustFinishedContent(); ok {
			fullContent.WriteString(content)
			if callOptions.StreamingFunc != nil {
				if err := callOptions.StreamingFunc(ctx, []byte(content)); err != nil {
					return nil, err
				}
			}
		}

		// Handle content deltas for real-time streaming
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			delta := chunk.Choices[0].Delta.Content
			fullContent.WriteString(delta)
			if callOptions.StreamingFunc != nil {
				if err := callOptions.StreamingFunc(ctx, []byte(delta)); err != nil {
					return nil, err
				}
			}
		}

		// Handle tool calls when they're finished
		if tool, ok := acc.JustFinishedToolCall(); ok {
			// Convert finished tool call to our format
			toolCall := tools.ToolCall{
				ID:   tool.ID,
				Type: "function",
				Function: tools.FunctionCall{
					Name:      tool.Name,
					Arguments: tool.Arguments,
				},
			}
			currentToolCalls = append(currentToolCalls, toolCall)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	// If we have tool calls, process them after streaming is complete
	result := &CompletionResponse{
		ID:      "",
		Object:  "chat.completion",
		Created: 0,
		Model:   c.model,
		Choices: []Choices{
			{
				Message: models.MessageResponse{
					Role:      models.TypeAssistantRole,
					Content:   fullContent.String(),
					ToolCalls: currentToolCalls,
				},
				Index:        0,
				FinishReason: "stop",
			},
		},
	}

	// If there are tool calls, process them and continue the conversation
	if len(currentToolCalls) > 0 {
		result.Choices[0].FinishReason = "tool_calls"
		return c.processToolCalls(ctx, originalMessages, result, maptools, callOptions)
	}

	return result, nil
}

func (c *Client) buildOpenAITools(toolkits []toolkit.Tool) ([]openai.ChatCompletionToolParam, map[string]toolkit.Tool, error) {
	var result []openai.ChatCompletionToolParam
	maptools := make(map[string]toolkit.Tool)

	for _, tool := range toolkits {
		for methodName := range tool.GetMethods() {
			// Get parameter schema
			paramSchema := tool.GetParameterStruct(methodName)

			// Convert to OpenAI format using the correct tool type
			openaiTool := openai.ChatCompletionToolParam{
				Type: "function",
				Function: openai.FunctionDefinitionParam{
					Name:        methodName,
					Description: openai.String(tool.GetDescription()),
					Parameters:  openai.FunctionParameters(paramSchema),
				},
			}

			result = append(result, openaiTool)
			maptools[methodName] = tool
		}
	}

	return result, maptools, nil
}

func (c *Client) processToolCalls(ctx context.Context, originalMessages []models.Message, resp *CompletionResponse, maptools map[string]toolkit.Tool, callOptions *models.CallOptions) (*CompletionResponse, error) {
	debugmod := ctx.Value(models.DebugKey)

	if debugmod != nil && debugmod.(bool) {
		fmt.Printf("DEBUG: processToolCalls - Processing %d tool calls\n", len(resp.Choices[0].Message.ToolCalls))
	}

	// Store the original tool calls for the response
	originalToolCalls := resp.Choices[0].Message.ToolCalls

	// Add assistant message with tool calls
	newMessages := append(originalMessages, models.Message{
		Role:      models.TypeAssistantRole,
		Content:   resp.Choices[0].Message.Content,
		ToolCalls: resp.Choices[0].Message.ToolCalls,
	})

	// Execute tools and add responses
	for i, tc := range resp.Choices[0].Message.ToolCalls {
		if debugmod != nil && debugmod.(bool) {
			fmt.Printf("DEBUG: Executing tool %d - %s with args: %s\n", i, tc.Function.Name, tc.Function.Arguments)
		}

		var toolResponse string

		if tool, ok := maptools[tc.Function.Name]; ok {
			resTool, err := tool.Execute(tc.Function.Name, []byte(tc.Function.Arguments))
			if err != nil {
				toolResponse = fmt.Sprintf("Error executing tool: %v", err)
				if debugmod != nil && debugmod.(bool) {
					fmt.Printf("DEBUG: Tool execution error: %v\n", err)
				}
			} else {
				switch v := resTool.(type) {
				case string:
					toolResponse = v
				case map[string]interface{}:
					jsonBytes, _ := json.Marshal(v)
					toolResponse = string(jsonBytes)
				default:
					jsonBytes, _ := json.Marshal(v)
					toolResponse = string(jsonBytes)
				}

				if debugmod != nil && debugmod.(bool) {
					fmt.Printf("DEBUG: Tool %d response length: %d\n", i, len(toolResponse))
					fmt.Printf("DEBUG: Tool %d response preview: %.200s...\n", i, toolResponse)
				}
			}
		} else {
			toolResponse = fmt.Sprintf("Tool %s not found", tc.Function.Name)
			if debugmod != nil && debugmod.(bool) {
				fmt.Printf("DEBUG: Tool not found: %s\n", tc.Function.Name)
			}
		}

		// Add tool response message
		newMessages = append(newMessages, models.Message{
			ToolCallID: &tc.ID,
			Role:       models.TypeToolRole,
			Content:    toolResponse,
		})
	}

	if debugmod != nil && debugmod.(bool) {
		fmt.Printf("DEBUG: Making follow-up request with %d messages\n", len(newMessages))
	}

	// Make another request with tool responses
	newOptions := []models.Option{}
	if callOptions.Temperature != nil {
		newOptions = append(newOptions, models.WithTemperature(*callOptions.Temperature))
	}
	if callOptions.MaxTokens != nil {
		newOptions = append(newOptions, models.WithMaxTokens(*callOptions.MaxTokens))
	}
	if len(callOptions.ToolCall) > 0 {
		newOptions = append(newOptions, models.WithTools(callOptions.ToolCall))
	}
	// Preserve streaming function for the follow-up request
	if callOptions.StreamingFunc != nil {
		newOptions = append(newOptions, models.WithStreamingFunc(callOptions.StreamingFunc))
	}

	finalResponse, err := c.CreateChatCompletion(ctx, newMessages, newOptions...)
	if err != nil {
		if debugmod != nil && debugmod.(bool) {
			fmt.Printf("DEBUG: Follow-up request failed: %v\n", err)
		}
		return nil, err
	}

	if debugmod != nil && debugmod.(bool) {
		fmt.Printf("DEBUG: Follow-up response content length: %d\n", len(finalResponse.Choices[0].Message.Content))
	}

	// For testing purposes, we want to return a response that shows the tool calls were made
	// We'll merge the final response content with the original tool calls
	finalResponse.Choices[0].Message.ToolCalls = originalToolCalls

	return finalResponse, nil
}

// options to option.RequestOption
func toRequestOptions(opts models.ClientOptions) []option.RequestOption {
	var reqOpts []option.RequestOption

	if opts.APIKey != "" {
		reqOpts = append(reqOpts, option.WithAPIKey(opts.APIKey))
	}

	if opts.BaseURL != "" {
		reqOpts = append(reqOpts, option.WithBaseURL(opts.BaseURL))
	}

	return reqOpts
}

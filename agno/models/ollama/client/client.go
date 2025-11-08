package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
	"github.com/ollama/ollama/api"
)

var (
	FALSE = false
	TRUE  = true
)

// Client represents the client for the Ollama API
type Client struct {
	model string
	api   *api.Client
}

func NewClient(model, baseURL string, client *http.Client) *Client {
	url, _ := url.Parse(baseURL)
	api := api.NewClient(url, client)
	return &Client{
		model: model,
		api:   api,
	}
}

func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	// Get debug and tools flags from context
	debugmod := ctx.Value(models.DebugKey)
	showToolsCall := ctx.Value(models.ShowToolsCallKey)
	var msgs []api.Message

	//parse messages to msgs
	for _, msg := range messages {
		msgs = append(msgs, api.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	req := &api.ChatRequest{
		Model:    c.model,
		Messages: msgs,
		Stream:   &FALSE,
		Options:  make(map[string]interface{}),
	}

	// Process options
	callOptions := models.DefaultCallOptions()
	for _, option := range options {
		option(callOptions)
	}

	// Extract images from metadata and add to last user message
	if callOptions.Metadata != nil {
		if imagesInterface, ok := callOptions.Metadata["images"]; ok {
			if images, ok := imagesInterface.([]models.Image); ok && len(images) > 0 {
				// Convert images to base64 encoded bytes
				var base64Images []api.ImageData
				for _, img := range images {
					// If Data is populated with raw image bytes, encode to base64
					if len(img.Data) > 0 {
						// Ollama expects base64-encoded image data
						base64Str := base64.StdEncoding.EncodeToString(img.Data)
						base64Images = append(base64Images, api.ImageData(base64Str))
					} else if img.URL != "" {
						// URLs can be passed directly
						base64Images = append(base64Images, api.ImageData(img.URL))
					}
				}

				// Add images to the last user message
				if len(base64Images) > 0 && len(msgs) > 0 {
					// Find last user message
					for i := len(msgs) - 1; i >= 0; i-- {
						if msgs[i].Role == "user" {
							msgs[i].Images = base64Images
							break
						}
					}
				}
			}
		}
	}

	callOptions.Tools = nil

	opts, err := utils.StructToMap(callOptions)
	if err != nil {
		return nil, err
	}
	req.Options = opts

	_tools, maptools, _ := c.prepareTools(callOptions.ToolCall)
	req.Tools = _tools

	var responseTools []api.Message
	var resp_ api.ChatResponse
	if debugmod != nil && debugmod.(bool) {
		jsonDebugReq, _ := json.MarshalIndent(req, "", "  ")
		utils.DebugPanel(string(jsonDebugReq))
	}
	err = c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
		resp_ = resp
		if len(resp.Message.ToolCalls) == 0 {
			return nil
		}

		// Add assistant message with tool calls first
		responseTools = append(responseTools, api.Message{
			Role:      resp.Message.Role,
			Content:   resp.Message.Content,
			ToolCalls: resp.Message.ToolCalls,
		})

		// Process each tool call and add their responses
		for _, tc := range resp.Message.ToolCalls {
			if tool, ok := maptools[tc.Function.Name]; ok {
				args := tc.Function.Arguments

				if showToolsCall != nil && showToolsCall.(bool) {
					// Tool call start panel
					startTool := fmt.Sprintf("🚀 Running tool %s with args:", tc.Function.Name)
					utils.ToolCallPanel(startTool)
					argsJsonPanel, _ := json.MarshalIndent(args, "", "  ")
					utils.ToolCallPanel(string(argsJsonPanel))
				}

				// Convert back to JSON
				argsJSON, err := json.Marshal(args)
				if err != nil {
					return fmt.Errorf("error converting arguments to JSON: %v", err)
				}

				// Execute the tool with the corrected arguments
				resTool, err := tool.Execute(tc.Function.Name, argsJSON)
				if err != nil {
					return fmt.Errorf("error executing tool %s: %w", tc.Function.Name, err)
				}

				// Tool call completion panel
				if showToolsCall != nil && showToolsCall.(bool) {
					endTool := fmt.Sprintf("✅ Tool %s finished", tc.Function.Name)
					utils.ToolCallPanel(endTool)
				}

				// Convert tool result to string
				var toolResultStr string
				switch result := resTool.(type) {
				case string:
					toolResultStr = result
				case map[string]interface{}:
					// Convert map to JSON
					resultJSON, err := json.Marshal(result)
					if err != nil {
						return fmt.Errorf("error converting tool result to JSON: %w", err)
					}
					toolResultStr = string(resultJSON)
				default:
					// Try to convert any other type to JSON
					resultJSON, err := json.Marshal(result)
					if err != nil {
						return fmt.Errorf("error converting tool result to JSON: %w", err)
					}
					toolResultStr = string(resultJSON)
				}

				// Add tool response
				responseTools = append(responseTools, api.Message{
					Role:    "tool",
					Content: toolResultStr,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Add tool responses to request messages after tool execution
	if len(responseTools) > 0 {
		req.Messages = append(req.Messages, responseTools...)
	}

	// Reasoning: se contexto tiver reasoning true, usa API customizada
	if c.model == "deepseek-r1" || c.model == "qwq" || c.model == "qwen2.5-coder" || c.model == "openthinker" || strings.Contains(c.model, "qwen") {
		if ctx.Value("reasoning") == true {
			// Use all messages including tool responses for reasoning
			reasoningMsgs := req.Messages

			// Make a second request with thinking enabled
			thinkingReq := &api.ChatRequest{
				Model:    c.model,
				Messages: reasoningMsgs,
				Stream:   &FALSE,
				Options: map[string]interface{}{
					"think": true,
				},
			}

			var thinkingResp api.ChatResponse
			err = c.api.Chat(ctx, thinkingReq, func(resp api.ChatResponse) error {
				thinkingResp = resp
				return nil
			})
			if err != nil {
				return nil, err
			}

			return &CompletionResponse{
				Model: thinkingResp.Model,
				Message: ChatMessage{
					Role:     thinkingResp.Message.Role,
					Content:  thinkingResp.Message.Content,
					Thinking: "", // Ollama doesn't return thinking field directly
				},
				Done: thinkingResp.Done,
			}, nil
		}
	}

	// Always create a response from the initial response
	resp := resp_
	response := &CompletionResponse{
		Model:        resp.Model,
		EvalTime:     int64(resp.Metrics.EvalDuration),
		EvalCount:    resp.Metrics.EvalCount,
		PromptTime:   int64(resp.Metrics.PromptEvalDuration),
		PromptTokens: resp.Metrics.PromptEvalCount,
		TotalTime:    int64(resp.Metrics.TotalDuration),
		Message: ChatMessage{
			Role:     resp.Message.Role,
			Content:  resp.Message.Content,
			Thinking: "", // Will be populated from raw response if available
		},
		Done: resp.Done,
	}

	// If there were tool calls, make a second request to get the final response
	if len(resp_.Message.ToolCalls) > 0 {
		err = c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
			response = &CompletionResponse{
				Model:        resp.Model,
				EvalTime:     int64(resp.Metrics.EvalDuration),
				EvalCount:    resp.Metrics.EvalCount,
				PromptTime:   int64(resp.Metrics.PromptEvalDuration),
				PromptTokens: resp.Metrics.PromptEvalCount,
				TotalTime:    int64(resp.Metrics.TotalDuration),
				Message: ChatMessage{
					Role:     resp.Message.Role,
					Content:  resp.Message.Content,
					Thinking: "", // Will be populated from raw response if available
				},
				Done: resp.Done,
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return response, nil

}

func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Get debug and tools flags from context
	debugmod := ctx.Value(models.DebugKey)
	showToolsCall := ctx.Value(models.ShowToolsCallKey)
	var msgs []api.Message

	//parse messages to msgs
	for _, msg := range messages {
		msgs = append(msgs, api.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}

	req := &api.ChatRequest{
		Model:    c.model,
		Messages: msgs,
		Stream:   &TRUE,
	}

	// Process options
	callOptions := models.DefaultCallOptions()
	for _, option := range options {
		option(callOptions)
	}

	// Extract images from metadata and add to last user message
	if callOptions.Metadata != nil {
		if imagesInterface, ok := callOptions.Metadata["images"]; ok {
			if images, ok := imagesInterface.([]models.Image); ok && len(images) > 0 {
				// Convert images to base64 encoded bytes
				var base64Images []api.ImageData
				for _, img := range images {
					// If Data is populated with raw image bytes, encode to base64
					if len(img.Data) > 0 {
						// Ollama expects base64-encoded image data
						base64Str := base64.StdEncoding.EncodeToString(img.Data)
						base64Images = append(base64Images, api.ImageData(base64Str))
					} else if img.URL != "" {
						// URLs can be passed directly
						base64Images = append(base64Images, api.ImageData(img.URL))
					}
				}

				// Add images to the last user message
				if len(base64Images) > 0 && len(msgs) > 0 {
					// Find last user message
					for i := len(msgs) - 1; i >= 0; i-- {
						if msgs[i].Role == "user" {
							msgs[i].Images = base64Images
							break
						}
					}
				}
			}
		}
	}

	_tools, maptools, _ := c.prepareTools(callOptions.ToolCall)
	callOptions.Tools = nil
	req.Tools = _tools
	opts, err := utils.StructToMap(callOptions)
	if err != nil {
		return err
	}
	//remove ToolCall from options
	opts["ToolCall"] = nil
	req.Options = opts

	if len(_tools) > 0 {
		var responseTools []api.Message

		err = c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
			if resp.Done {
				return nil
			}
			// Process each tool call
			for _, tc := range resp.Message.ToolCalls {
				if tool, ok := maptools[tc.Function.Name]; ok {
					args := tc.Function.Arguments

					if showToolsCall != nil && showToolsCall.(bool) {
						// Tool call start panel
						argsJsonPanel, _ := json.MarshalIndent(args, "", "  ")
						startTool := fmt.Sprintf("🚀 Running tool %s with args:", tc.Function.Name)
						utils.ToolCallPanel(startTool)
						utils.ToolCallPanel(string(argsJsonPanel))
					}

					// Convert back to JSON
					argsJSON, err := json.Marshal(args)
					if err != nil {
						return fmt.Errorf("error converting arguments to JSON: %w", err)
					}

					// Execute the tool with the corrected arguments
					resTool, err := tool.Execute(tc.Function.Name, argsJSON)
					if err != nil {
						return fmt.Errorf("error executing tool %s: %w", tc.Function.Name, err)
					}

					// Tool call completion panel
					if showToolsCall != nil && showToolsCall.(bool) {
						endTool := fmt.Sprintf("✅ Tool %s finished", tc.Function.Name)
						utils.ToolCallPanel(endTool)
					}

					// Convert tool result to string
					var toolResultStr string
					switch result := resTool.(type) {
					case string:
						toolResultStr = result
					case map[string]interface{}:
						// Convert map to JSON
						resultJSON, err := json.Marshal(result)
						if err != nil {
							return fmt.Errorf("error converting tool result to JSON: %w", err)
						}
						toolResultStr = string(resultJSON)
					default:
						// Try to convert any other type to JSON
						resultJSON, err := json.Marshal(result)
						if err != nil {
							return fmt.Errorf("error converting tool result to JSON: %w", err)
						}
						toolResultStr = string(resultJSON)
					}

					// Convert tool response to JSON
					// Add tool response to the response list
					responseTools = append(responseTools, api.Message{
						Role:    "tool",
						Content: toolResultStr,
					})

					req.Messages = append(req.Messages, responseTools...)
				}
			}

			return nil
		})

		if err != nil {
			return err
		}
	}

	var buffer strings.Builder
	//lastFlush := time.Now()
	req.Tools = nil
	if debugmod != nil && debugmod.(bool) {
		jsonDebugReq, _ := json.MarshalIndent(req, "", "  ")
		utils.DebugPanel(string(jsonDebugReq))
	}

	err = c.api.Chat(ctx, req, func(resp api.ChatResponse) error {

		if resp.Message.Content != "" {
			buffer.WriteString(resp.Message.Content)
			if stopSentence(resp.Message.Content) {
				callOptions.StreamingFunc(ctx, []byte(buffer.String()))
				buffer.Reset()
				//	lastFlush = time.Now()

				fmt.Println(buffer.String())
			}
		}

		return nil
	})

	// add last response in req
	if buffer.Len() > 0 {
		msg := api.Message{
			Role:    "assistant",
			Content: buffer.String(),
		}
		req.Messages = append(req.Messages, msg)
	}

	return err

}

func stopSentence(text string) bool {
	return strings.HasSuffix(text, ".") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, "!") || strings.HasSuffix(text, "\n") || strings.HasSuffix(text, ":")
}

func (c *Client) prepareTools(toolsCall []toolkit.Tool) ([]api.Tool, map[string]toolkit.Tool, []string) {
	var apiTools []api.Tool
	maptools := make(map[string]toolkit.Tool)
	var names []string

	for _, tool := range toolsCall {
		for methodName := range tool.GetMethods() {
			// Get parameter schema
			params := tool.GetParameterStruct(methodName)

			// Extract properties and required fields
			propsMap, ok := params["properties"].(map[string]interface{})
			if !ok {
				fmt.Printf("⚠️ 'properties' is not a map[string]interface{} for method '%s'\n", methodName)
				continue
			}

			requiredFields := []string{}
			if req, ok := params["required"].([]string); ok {
				requiredFields = req
			}

			// Build properties map for Ollama
			ollamaProps := make(map[string]api.ToolProperty)

			for propName, propValue := range propsMap {
				propObj, ok := propValue.(map[string]interface{})
				if !ok {
					fmt.Printf("⚠️ Property '%s' is not a map[string]interface{}\n", propName)
					continue
				}

				typeStr := "string"
				if t, ok := propObj["type"].(string); ok {
					typeStr = strings.ToLower(t)
				}

				description := ""
				if d, ok := propObj["description"].(string); ok {
					description = d
				}

				var enumVals []any
				if e, ok := propObj["enum"].([]interface{}); ok {
					enumVals = e
				}

				ollamaProps[propName] = api.ToolProperty{
					Type:        api.PropertyType([]string{typeStr}),
					Description: description,
					Enum:        enumVals,
				}
			}

			// Define parameters in the format expected by Ollama
			// Add the tool to the list
			apiTools = append(apiTools, api.Tool{
				Type: "function",
				Function: api.ToolFunction{
					Name:        methodName,
					Description: tool.GetDescription(),
					Parameters: api.ToolFunctionParameters{
						Type:       "object",
						Required:   requiredFields,
						Properties: ollamaProps,
					},
				},
			})

			maptools[methodName] = tool
			names = append(names, methodName)
		}
	}

	return apiTools, maptools, names
}

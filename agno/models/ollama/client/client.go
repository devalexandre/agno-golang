package client

import (
	"context"
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

	callOptions.Tools = nil

	opts, err := utils.StructToMap(callOptions)
	if err != nil {
		return nil, err
	}

	// Convert max_tokens to num_predict for Ollama API
	if val, ok := opts["max_tokens"]; ok && val != nil {
		opts["num_predict"] = val
		delete(opts, "max_tokens")
	}
	mergeRequestParams(opts, callOptions.RequestParams)
	applyThink(req, opts)

	req.Options = opts

	_tools, maptools, _ := c.prepareTools(callOptions.ToolCall)
	req.Tools = _tools

	if showToolsCall != nil && showToolsCall.(bool) {
		toolsJosn, _ := json.MarshalIndent(_tools, "", "  ")
		utils.ToolCallPanel(string(toolsJosn))
	}

	var responseTools []api.Message
	var resp_ api.ChatResponse
	if debugmod != nil && debugmod.(bool) {
		jsonDebugReq, _ := json.MarshalIndent(req, "", "  ")
		utils.DebugPanel(string(jsonDebugReq))
	}

	// Store tool results for returning to caller
	var toolResults []models.ToolResult

	// The model may need several rounds of tool calls (read a file, then
	// another, then answer): keep asking until it answers in text.
	for round := 0; ; round++ {
		responseTools = nil
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

					// Capture tool result for returning to caller
					toolResult := models.ToolResult{
						ToolName:  tc.Function.Name,
						ToolInput: string(argsJSON),
						Result:    resTool,
					}
					if err != nil {
						// A failed or refused tool call goes back to the model as
						// the tool's answer, so it can explain or try another way.
						toolResult.Error = err.Error()
						toolResults = append(toolResults, toolResult)
						responseTools = append(responseTools, api.Message{
							Role:    "tool",
							Content: "error: " + err.Error(),
						})
						continue
					}
					toolResults = append(toolResults, toolResult)

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
				} else {
					responseTools = append(responseTools, api.Message{
						Role:    "tool",
						Content: "error: unknown tool " + tc.Function.Name,
					})
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
		if len(resp_.Message.ToolCalls) == 0 {
			break
		}
		// Add tool responses to request messages after tool execution
		req.Messages = append(req.Messages, responseTools...)
		if round+1 >= maxToolRounds {
			// One last request without tools forces a text answer.
			req.Tools = nil
			err = c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
				resp_ = resp
				return nil
			})
			if err != nil {
				return nil, err
			}
			break
		}
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

	// Some thinking models stop right after reasoning and leave the answer
	// empty. The reasoning then holds the answer: better to return it than
	// an empty reply.
	if strings.TrimSpace(resp_.Message.Content) == "" && len(resp_.Message.ToolCalls) == 0 {
		resp_.Message.Content = strings.TrimSpace(resp_.Message.Thinking)
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
		Done:        resp.Done,
		ToolResults: toolResults, // Include tool results
	}

	return response, nil

}

func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) error {
	// Get debug and tools flags from context
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

	_tools, maptools, _ := c.prepareTools(callOptions.ToolCall)
	callOptions.Tools = nil
	req.Tools = _tools
	opts, err := utils.StructToMap(callOptions)
	if err != nil {
		return err
	}

	// Convert max_tokens to num_predict for Ollama API
	if val, ok := opts["max_tokens"]; ok && val != nil {
		opts["num_predict"] = val
		delete(opts, "max_tokens")
	}
	mergeRequestParams(opts, callOptions.RequestParams)
	applyThink(req, opts)

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

	err = c.api.Chat(ctx, req, func(resp api.ChatResponse) error {
		if resp.Message.Content != "" {
			if err := callOptions.StreamingFunc(ctx, []byte(resp.Message.Content)); err != nil {
				return err
			}
		}
		return nil
	})

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
		tooname := tool.GetName()
		if strings.Contains(tooname, "_") {
			fmt.Printf("⚠️ Tool name '%s' contains underscores. It's recommended to use camelCase names for Ollama compatibility.\n", tooname)
			panic("Name Tool can not be Underscore")
		}
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
					Description: tool.GetDescriptionOfMethod(methodName),
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

// mergeRequestParams copies provider-specific request parameters (num_ctx,
// num_gpu, repeat_penalty, ...) into the Ollama options map. Without it they
// would travel nested under "request_params", a key Ollama ignores, and a
// setting such as the context size would never reach the server.
func mergeRequestParams(opts map[string]interface{}, params map[string]interface{}) {
	delete(opts, "request_params")
	for k, v := range params {
		opts[k] = v
	}
}

// maxToolRounds caps how many times in a row the model may ask for tools
// before it has to answer.
const maxToolRounds = 32

// applyThink moves a "think" request parameter (true/false) out of the
// options map, where Ollama would ignore it, to the request field that turns
// a thinking model's reasoning on or off.
func applyThink(req *api.ChatRequest, opts map[string]interface{}) {
	v, ok := opts["think"]
	if !ok {
		return
	}
	delete(opts, "think")
	if b, ok := v.(bool); ok {
		req.Think = &api.ThinkValue{Value: b}
	}
}

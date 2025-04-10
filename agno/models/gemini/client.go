package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/utils"
	"google.golang.org/genai"
)

// Client represents the Gemini API client
type Client struct {
	model       string
	apiKey      string
	genaiClient *genai.Client
	options     ClientOptions
}

// OptionClient defines options for the client
type OptionClient func(*ClientOptions)

// NewClient creates a new Gemini client
func NewClient(options ...OptionClient) (*Client, error) {
	opts := ClientOptions{}
	for _, option := range options {
		option(&opts)
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is not set")
		}
	}

	if opts.Model == "" {
		opts.Model = "gemini-2.0-flash"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Client{
		model:       opts.Model,
		apiKey:      apiKey,
		genaiClient: client,
		options:     opts,
	}, nil
}

// CreateChatCompletion implements simple chat completion (non-streaming)
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	callOptions := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOptions)
	}

	// Prepare tools if any
	functionDeclarations, maptools, names := c.prepareTools(callOptions.ToolCall)

	// Prepare content (messages)
	contents := toContents(messages)

	// Initial system prompt
	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			{Text: "You're a helpful assistant. When you receive a tool response, always use the tool result to answer the user's original question."},
		},
	}

	// Prepare configuration
	config := &genai.GenerateContentConfig{
		SystemInstruction: systemInstruction,
		Temperature:       callOptions.Temperature,
		TopP:              callOptions.TopP,
		FrequencyPenalty:  callOptions.FrequencyPenalty,
		PresencePenalty:   callOptions.PresencePenalty,
	}

	// Add tools if declared
	if len(functionDeclarations) > 0 {
		config.Tools = []*genai.Tool{{FunctionDeclarations: functionDeclarations}}
		config.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: names,
			},
		}
	}

	// Execute request
	resp, err := c.genaiClient.Models.GenerateContent(ctx, c.model, contents, config)
	if err != nil {
		return nil, err
	}

	// Check tool call
	if len(resp.FunctionCalls()) > 0 {
		toolCall := resp.FunctionCalls()[0]
		tool, ok := maptools[toolCall.Name]
		if !ok {
			return nil, fmt.Errorf("tool %q not found", toolCall.Name)
		}

		args, err := json.Marshal(toolCall.Args)
		if err != nil {
			return nil, err
		}

		toolResult, err := tool.Execute(args)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		if c.options.Debug {
			utils.CreateToolCallPanel(toolResult.(string), 0)
		}

		// Final answer with tool result
		resultContents := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{{
					Text: fmt.Sprintf("The result of tool %s is: %v", toolCall.Name, toolResult),
				}},
			},
		}

		finalResp, err := c.genaiClient.Models.GenerateContent(ctx, c.model, resultContents, nil)
		if err != nil {
			return nil, err
		}

		return buildCompletionResponse(c.model, finalResp), nil
	}

	// Normal flow
	return buildCompletionResponse(c.model, resp), nil
}

// StreamChatCompletion streams responses
func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan models.MessageResponse, error) {
	callOptions := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOptions)
	}

	functionDeclarations, maptools, names := c.prepareTools(callOptions.ToolCall)

	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			{Text: "You're a helpful assistant. When you receive a tool response, always use the tool result to answer the user's original question."},
		},
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: systemInstruction,
		Temperature:       callOptions.Temperature,
		TopP:              callOptions.TopP,
		FrequencyPenalty:  callOptions.FrequencyPenalty,
		PresencePenalty:   callOptions.PresencePenalty,
	}

	// Add tools if declared
	if len(functionDeclarations) > 0 {
		config.Tools = []*genai.Tool{{FunctionDeclarations: functionDeclarations}}
		config.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: names,
			},
		}
	}

	// Convert messages to contents for the API
	contents := toContents(messages)

	// Create response channel
	responseChannel := make(chan models.MessageResponse)

	go func() {
		defer close(responseChannel)

		// Send the initial request and process the stream
		var fullResponse string
		var toolCallDetected bool

		for chunk, err := range c.genaiClient.Models.GenerateContentStream(ctx, c.model, contents, config) {
			if err != nil {
				fmt.Printf("Error reading from stream: %v\n", err)
				break
			}

			// Check for function calls
			if len(chunk.FunctionCalls()) > 0 && !toolCallDetected {
				toolCallDetected = true
				toolCall := chunk.FunctionCalls()[0]
				tool, ok := maptools[toolCall.Name]
				if !ok {
					fmt.Printf("Tool %q not found\n", toolCall.Name)
					continue
				}

				// Execute the tool
				args, err := json.Marshal(toolCall.Args)
				if err != nil {
					fmt.Printf("Failed to marshal tool args: %v\n", err)
					continue
				}

				toolResult, err := tool.Execute(args)
				if err != nil {
					fmt.Printf("Tool execution failed: %v\n", err)
					continue
				}

				if c.options.Debug {
					utils.CreateToolCallPanel(toolResult.(string), 0)
				}

				// Send tool result to the channel
				toolResultMsg := fmt.Sprintf("Tool %s result: %v", toolCall.Name, toolResult)
				responseChannel <- models.MessageResponse{
					Role:    string(models.TypeAssistantRole),
					Content: toolResultMsg,
				}

				if callOptions.StreamingFunc != nil {
					callOptions.StreamingFunc(ctx, []byte(toolResultMsg))
				}

				// Send the tool result back to the model for a final response
				feedbackContent := []*genai.Content{
					{
						Role: "user",
						Parts: []*genai.Part{{
							Text: fmt.Sprintf("The result of tool %s is: %v", toolCall.Name, toolResult),
						}},
					},
				}

				// Get final response with tool result and process the stream
				for finalChunk, err := range c.genaiClient.Models.GenerateContentStream(ctx, c.model, feedbackContent, nil) {
					if err != nil {
						fmt.Printf("Error reading from final stream: %v\n", err)
						break
					}

					if finalChunk.Text() != "" {
						if callOptions.StreamingFunc != nil {
							callOptions.StreamingFunc(ctx, []byte(finalChunk.Text()))
						}
						responseChannel <- models.MessageResponse{
							Role:    string(models.TypeAssistantRole),
							Content: finalChunk.Text(),
						}
					}
				}
				return // End after tool execution and final response
			}

			// Regular text response
			if chunk.Text() != "" {
				fullResponse += chunk.Text()
				if callOptions.StreamingFunc != nil {
					callOptions.StreamingFunc(ctx, []byte(chunk.Text()))
				}
				responseChannel <- models.MessageResponse{
					Role:    string(models.TypeAssistantRole),
					Content: chunk.Text(),
				}
			}
		}
	}()

	return responseChannel, nil
}

// Helper: convert messages to contents
func toContents(messages []models.Message) []*genai.Content {
	var contents []*genai.Content
	for _, msg := range messages {
		contents = append(contents, &genai.Content{
			Role:  string(msg.Role),
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}
	return contents
}

// Prepares tools
func (c *Client) prepareTools(toolsCall []tools.Tool) ([]*genai.FunctionDeclaration, map[string]tools.Tool, []string) {
	var functionDeclarations []*genai.FunctionDeclaration
	maptools := make(map[string]tools.Tool)
	var names []string

	for _, tool := range toolsCall {
		params, _ := tools.GenerateJSONSchema(tool.GetParameterStruct())
		schema := &genai.Schema{
			Type: genai.TypeObject,
		}

		if propsMap, ok := params["properties"].(map[string]any); ok {
			schema.Properties = make(map[string]*genai.Schema)

			// Extract required properties if available
			var requiredProps []string
			if requiredArr, ok := params["required"].([]any); ok {
				for _, req := range requiredArr {
					if reqStr, ok := req.(string); ok {
						requiredProps = append(requiredProps, reqStr)
					}
				}
			}

			for propName, propValue := range propsMap {
				if propObj, ok := propValue.(map[string]any); ok {
					typeStr := "string" // Default type
					if typeVal, ok := propObj["type"]; ok {
						if typeStr, ok = typeVal.(string); !ok {
							typeStr = "string" // Fallback to string if type is not a string
						}
					}

					description := ""
					if descVal, ok := propObj["description"]; ok {
						if descStr, ok := descVal.(string); ok {
							description = descStr
						}
					}

					schema.Properties[propName] = &genai.Schema{
						Type:        parseSchemaType(typeStr),
						Description: description,
					}

					// Check if this property is in the required list
					isRequired := false
					for _, req := range requiredProps {
						if req == propName {
							isRequired = true
							break
						}
					}
					// Note: We could use slices.Contains in Go 1.21+, but keeping compatibility with older Go versions

					if isRequired {
						schema.Required = append(schema.Required, propName)
					}
				}
			}
		}

		functionDeclarations = append(functionDeclarations, &genai.FunctionDeclaration{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  schema,
		})

		maptools[tool.Name()] = tool
		names = append(names, tool.Name())
	}

	return functionDeclarations, maptools, names
}

// Maps schema types
func parseSchemaType(typeStr string) genai.Type {
	switch typeStr {
	case "string":
		return genai.TypeString
	case "number", "integer":
		return genai.TypeNumber
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}

// Builds completion response
func buildCompletionResponse(model string, resp *genai.GenerateContentResponse) *CompletionResponse {
	return &CompletionResponse{
		ID:      "",
		Object:  "chat.completion",
		Created: 0,
		Model:   model,
		Choices: []Choices{{
			Index: 0,
			Message: models.MessageResponse{
				Role:    string(models.TypeAssistantRole),
				Content: resp.Text(),
			},
			FinishReason: "stop",
		}},
	}
}

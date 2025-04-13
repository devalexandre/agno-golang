package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
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

	if opts.ID == "" {
		opts.ID = "gemini-2.0-flash"
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
		model:       opts.ID,
		apiKey:      apiKey,
		genaiClient: client,
		options:     opts,
	}, nil
}

// CreateChatCompletion implements simple chat completion (non-streaming)
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	//debug system instruction
	debugmod := ctx.Value(models.DebugKey)
	showToolsCall := ctx.Value(models.ShowToolsCallKey)

	callOptions := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOptions)
	}

	// Prepare tools if any
	functionDeclarations, maptools, names := c.prepareTools(callOptions.ToolCall)

	// Initial system prompt (se existir)
	var systemInstruction *genai.Content
	// get system instruction in messages (se existir)
	for _, msg := range messages {
		if msg.Role == models.TypeSystemRole {
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
			}
		}
	}

	//remove system message from the message list
	messages = removeSystemMessage(messages)

	// Prepare content (messages)
	contents := toContents(messages)

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
	//shwo debug user instruction
	if debugmod != nil && debugmod.(bool) {
		debug := "[Prompt] \n"
		debug += contents[len(contents)-1].Parts[0].Text + "\n"

		debug = "\n[System Instruction]\n"
		debug += systemInstruction.Parts[0].Text + "\n"
		utils.DebugPanel(debug)
	}

	// if showToolsCall != nil && showToolsCall.(bool) {
	// 	jsonTools, _ := json.MarshalIndent(config.Tools, "", "  ")
	// 	debug := "\n[Tools]\n" + string(jsonTools) + "\n"
	// 	utils.ToolCallPanel(debug)
	// }

	resp, err := c.genaiClient.Models.GenerateContent(ctx, c.model, contents, config)
	if err != nil {
		return nil, err
	}

	var resultContents []*genai.Content

	// Check tool call
	if len(resp.FunctionCalls()) > 0 {
		for _, toolCall := range resp.FunctionCalls() {

			tool, ok := maptools[toolCall.Name]
			if !ok {
				return nil, fmt.Errorf("tool %q not found", toolCall.Name)
			}
			// Convert tool arguments map[string]interface {} to JSON
			args, err := json.Marshal(toolCall.Args)
			if err != nil {
				return nil, err
			}

			if showToolsCall != nil && showToolsCall.(bool) {
				debug := fmt.Sprintf("Tool %s \n", toolCall.Name)
				debug += fmt.Sprintf("Args: %s \n", string(args))
				utils.ToolCallPanel(debug)
			}

			// Execute the tool
			toolResult, err := tool.Execute(toolCall.Name, args)
			if err != nil {
				return nil, fmt.Errorf("tool execution failed: %w", err)
			}

			resultContents = append(resultContents, &genai.Content{
				Role: "user",
				Parts: []*genai.Part{{
					Text: fmt.Sprintf("The result of tool %s is: %v", toolCall.Name, toolResult),
				}},
			})

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
	debugmod := ctx.Value(models.DebugKey)
	showToolsCall := ctx.Value(models.ShowToolsCallKey)

	callOptions := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOptions)
	}

	functionDeclarations, maptools, names := c.prepareTools(callOptions.ToolCall)

	var systemInstruction *genai.Content
	for _, msg := range messages {
		if msg.Role == models.TypeSystemRole {
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
			}
		}
	}
	messages = removeSystemMessage(messages)

	if debugmod != nil && debugmod.(bool) {
		debug := "[Prompt] \n" + systemInstruction.Parts[0].Text + "\n"
		utils.DebugPanel(debug)

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

		var fullResponse string
		var resultContents []*genai.Content

		for chunk, err := range c.genaiClient.Models.GenerateContentStream(ctx, c.model, contents, config) {
			if err != nil {
				fmt.Printf("Error reading from stream: %v\n", err)
				break
			}

			// ✅ Processa todas as tools no chunk
			if len(chunk.FunctionCalls()) > 0 {
				for _, toolCall := range chunk.FunctionCalls() {
					tool, ok := maptools[toolCall.Name]
					if !ok {
						fmt.Printf("Tool %q not found\n", toolCall.Name)
						continue
					}

					args, err := json.Marshal(toolCall.Args)
					if err != nil {
						fmt.Printf("Failed to marshal tool args: %v\n", err)
						continue
					}

					if showToolsCall != nil && showToolsCall.(bool) {
						startTool := fmt.Sprintf("🚀 Running tool %s with args: %s", toolCall.Name, string(args))
						utils.ToolCallPanel(startTool)

					}

					toolResult, err := tool.Execute(toolCall.Name, args)
					if err != nil {
						fmt.Printf("Tool execution failed: %v\n", err)
						continue
					}

					// Acumula os resultados
					resultContents = append(resultContents, &genai.Content{
						Role: "user",
						Parts: []*genai.Part{{
							Text: fmt.Sprintf("The result of tool %s is: %v", toolCall.Name, toolResult),
						}},
					})

					// Também envia para o canal de stream imediatamente (opcional)
					toolResultMsg := fmt.Sprintf("Tool %s result: %v", toolCall.Name, toolResult)
					responseChannel <- models.MessageResponse{
						Model:   c.model,
						Role:    string(models.TypeAssistantRole),
						Content: toolResultMsg,
					}
					if callOptions.StreamingFunc != nil {
						callOptions.StreamingFunc(ctx, []byte(toolResultMsg))
					}

					if showToolsCall != nil && showToolsCall.(bool) {
						endTool := fmt.Sprintf("✅ Tool %s finished", toolCall.Name)
						utils.ToolCallPanel(endTool)
					}

				}

				// Depois de processar todas as tools, gera a resposta final
				if len(resultContents) > 0 {
					finalStream := c.genaiClient.Models.GenerateContentStream(ctx, c.model, resultContents, nil)
					for finalChunk, err := range finalStream {
						if err != nil {
							fmt.Printf("Error reading from final stream: %v\n", err)
							break
						}

						if finalChunk.Text() != "" {
							if callOptions.StreamingFunc != nil {
								callOptions.StreamingFunc(ctx, []byte(finalChunk.Text()))
							}
							responseChannel <- models.MessageResponse{
								Model:   c.model,
								Role:    string(models.TypeAssistantRole),
								Content: finalChunk.Text(),
							}
						}
					}
				}

				continue // continua o loop para novos chunks
			}

			// ✅ Resposta normal do modelo
			if chunk.Text() != "" {
				fullResponse += chunk.Text()
				if callOptions.StreamingFunc != nil {
					callOptions.StreamingFunc(ctx, []byte(chunk.Text()))
				}
				responseChannel <- models.MessageResponse{
					Model:   c.model,
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

func (c *Client) prepareTools(toolsCall []toolkit.Tool) ([]*genai.FunctionDeclaration, map[string]toolkit.Tool, []string) {
	var functionDeclarations []*genai.FunctionDeclaration
	maptools := make(map[string]toolkit.Tool)
	var names []string

	for _, tool := range toolsCall {
		for methodName := range tool.GetMethods() {

			// ✅ Pega o schema da função já gerado no toolkit
			params := tool.GetParameterStruct(methodName)

			propsMap, ok := params["properties"].(map[string]interface{})
			if !ok {
				fmt.Printf("⚠️ params['properties'] is not a map[string]interface{}\n")
				continue
			}

			schema := &genai.Schema{
				Type:       genai.TypeObject,
				Properties: make(map[string]*genai.Schema),
				Required:   []string{},
			}

			// ✅ Extrai required
			var requiredProps []string
			if requiredArr, ok := params["required"].([]string); ok {
				requiredProps = requiredArr
			}

			// ✅ Mapeia todas as propriedades
			for propName, propValue := range propsMap {
				propObj, ok := propValue.(map[string]interface{})
				if !ok {
					fmt.Printf("⚠️ propObj is not map[string]interface{} for field '%s'\n", propName)
					continue
				}

				typeStr := "string"
				if typeVal, ok := propObj["type"].(string); ok {
					typeStr = strings.ToLower(typeVal)
				}

				description := ""
				if descVal, ok := propObj["description"].(string); ok {
					description = descVal
				}

				fieldSchema := &genai.Schema{
					Type:        parseSchemaType(typeStr),
					Description: description,
				}

				// ✅ Tratamento especial para arrays
				if typeStr == "array" {
					if itemsValRaw, ok := propObj["items"]; ok {
						if itemsVal, ok := itemsValRaw.(map[string]interface{}); ok {
							if itemTypeVal, ok := itemsVal["type"].(string); ok {
								fieldSchema.Items = &genai.Schema{
									Type: parseSchemaType(strings.ToLower(itemTypeVal)),
								}
							}
						} else {
							fmt.Printf("⚠️ Missing 'items' for array field '%s'\n", propName)
							continue
						}
					}
				}

				schema.Properties[propName] = fieldSchema
				schema.Required = requiredProps

			}

			// ✅ Cria a declaração da função final
			functionDeclarations = append(functionDeclarations, &genai.FunctionDeclaration{
				Name:        methodName,
				Description: tool.GetDescription(),
				Parameters:  schema,
			})

			maptools[methodName] = tool

			names = append(names, methodName)
		}
	}

	return functionDeclarations, maptools, names
}

func safeConvertMap(input interface{}) map[string]any {
	result := make(map[string]any)

	switch inputMap := input.(type) {

	case map[string]any:
		return inputMap

	case map[string]string:
		for k, v := range inputMap {
			result[k] = v
		}
		return result

	case map[string]float64:
		for k, v := range inputMap {
			result[k] = v
		}
		return result

	case map[string]bool:
		for k, v := range inputMap {
			result[k] = v
		}
		return result

	case map[interface{}]interface{}:
		for k, v := range inputMap {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = v
			}
		}
		return result

	case map[interface{}]string:
		for k, v := range inputMap {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = v
			}
		}
		return result

	case map[interface{}]float64:
		for k, v := range inputMap {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = v
			}
		}
		return result

	case map[interface{}]bool:
		for k, v := range inputMap {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = v
			}
		}
		return result

	default:
		fmt.Printf("⚠️ safeConvertMap: unexpected type %T\n", input)
	}

	return result
}

func convertMapInterfaceToString(input interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	switch inputMap := input.(type) {
	case map[string]any:
		for k, v := range inputMap {
			result[k] = normalizeValue(v)
		}
	case map[string]string:
		for k, v := range inputMap {
			result[k] = v
		}
	case map[interface{}]interface{}:
		for k, v := range inputMap {
			if keyStr, ok := k.(string); ok {
				result[keyStr] = normalizeValue(v)
			}
		}
	default:
		fmt.Printf("⚠️ convertMapInterfaceToString: unexpected type %T\n", input)
	}

	return result
}

func normalizeValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return convertMapInterfaceToString(v)
	case map[string]string:
		return convertMapInterfaceToString(v)
	case map[interface{}]interface{}:
		return convertMapInterfaceToString(v)
	case []interface{}:
		for i := range v {
			v[i] = normalizeValue(v[i])
		}
		return v
	default:
		return v
	}
}

// Maps schema types
func parseSchemaType(typeStr string) genai.Type {
	// Normalize para lowercase
	switch strings.ToLower(typeStr) {
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
		ID:      resp.ResponseID,
		Object:  "chat.completion",
		Created: 0,
		Model:   model,
		Choices: []Choices{{
			Index: 0,
			Message: models.MessageResponse{
				Model:   model,
				Role:    string(models.TypeAssistantRole),
				Content: resp.Text(),
			},
			FinishReason: "stop",
		}},
	}
}

func removeSystemMessage(messages []models.Message) []models.Message {
	var filteredMessages []models.Message
	for _, msg := range messages {
		if msg.Role != models.TypeSystemRole {
			filteredMessages = append(filteredMessages, msg)
		}
	}
	return filteredMessages
}

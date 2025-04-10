package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
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
		opts.Model = "gemini-2.0-pro"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
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

// CreateChatCompletion handles chat completion with tools support
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	callOptions := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOptions)
	}

	functionDeclarations, maptools := c.prepareTools(callOptions.ToolCall)

	model := c.genaiClient.GenerativeModel(c.model)
	if len(functionDeclarations) > 0 {
		model.Tools = []*genai.Tool{{FunctionDeclarations: functionDeclarations}}

	}

	// Start chat session
	session := model.StartChat()

	// Step 0: User prompt to guide model behavior
	session.History = append(session.History, &genai.Content{
		Role: models.TypeAssistantRole,
		Parts: []genai.Part{
			genai.Text("When you receive a tool response, always use the tool result to answer the user's original question."),
		},
	})

	// Step 1: Add user message to history
	userMessage := messages[len(messages)-1].Content

	// Step 2: Send user message
	initialResp, err := session.SendMessage(ctx, genai.Text(userMessage))
	if err != nil {
		return nil, fmt.Errorf("failed to send initial message: %w", err)
	}

	if len(initialResp.Candidates) == 0 {
		return emptyCompletionResponse(c.model), nil
	}

	candidate := initialResp.Candidates[0]
	toolCall, hasToolCall := extractToolCall(candidate)

	if !hasToolCall {
		return buildCompletionResponse(c.model, candidate, nil), nil
	}

	// Step 3: Execute tool
	tool, ok := maptools[toolCall.Function.Name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", toolCall.Function.Name)
	}

	toolResult, err := tool.Execute([]byte(toolCall.Function.Arguments))
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	resultMap, err := ConvertJSONToMap(toolResult.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to convert tool result to map: %w", err)
	}

	// Step 5: Final response
	// Send tool response directly into session
	finalResp, err := session.SendMessage(ctx, genai.FunctionResponse{
		Name:     toolCall.Function.Name,
		Response: resultMap,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send tool response: %w", err)
	}

	return buildCompletionResponse(c.model, finalResp.Candidates[0], &toolCall), nil

}

// StreamChatCompletion handles chat completion with streaming support
func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan models.MessageResponse, error) {
	callOptions := models.DefaultCallOptions()
	for _, opt := range options {
		opt(callOptions)
	}

	// Prepara ferramentas e declarações
	functionDeclarations, maptools := c.prepareTools(callOptions.ToolCall)

	model := c.genaiClient.GenerativeModel(c.model)
	if len(functionDeclarations) > 0 {
		model.Tools = []*genai.Tool{{FunctionDeclarations: functionDeclarations}}
	}

	session := model.StartChat()

	// Instrução inicial para orientar o modelo
	session.History = append(session.History, &genai.Content{
		Role: models.TypeAssistantRole,
		Parts: []genai.Part{
			genai.Text("When you receive a tool response, always use the tool result to answer the user's original question."),
		},
	})

	userMessage := messages[len(messages)-1].Content

	// Cria canal para enviar mensagens streamadas
	responseChannel := make(chan models.MessageResponse)

	go func() {
		defer close(responseChannel)

		// Inicia o streaming da mensagem inicial
		iter := session.SendMessageStream(ctx, genai.Text(userMessage))

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				fmt.Printf("Stream error: %v\n", err)
				return
			}

			for _, candidate := range resp.Candidates {
				if candidate.Content != nil {
					for _, part := range candidate.Content.Parts {
						switch p := part.(type) {
						case genai.Text:
							text := string(p)
							if callOptions.StreamingFunc != nil {
								callOptions.StreamingFunc(ctx, []byte(text))
							}
							responseChannel <- models.MessageResponse{
								Role:    string(models.TypeAssistantRole),
								Content: text,
							}
						case genai.FunctionCall:
							// Tratamento de chamada da tool
							tool, ok := maptools[p.Name]
							if !ok {
								fmt.Printf("Tool %q not found\n", p.Name)
								return
							}

							args, err := json.Marshal(p.Args)
							if err != nil {
								fmt.Printf("Error marshaling args: %v\n", err)
								return
							}

							toolResult, err := tool.Execute(args)
							if err != nil {
								fmt.Printf("Tool execution error: %v\n", err)
								return
							}

							resultMap, err := ConvertJSONToMap(toolResult.(string))
							if err != nil {
								fmt.Printf("Failed to convert tool result: %v\n", err)
								return
							}

							// Agora, mandamos a resposta da tool no fluxo normal (não streaming interno!)
							finalResp, err := session.SendMessage(ctx, genai.FunctionResponse{
								Name:     p.Name,
								Response: resultMap,
							})
							if err != nil {
								fmt.Printf("Tool sendMessage error: %v\n", err)
								return
							}

							// Processa resposta final da tool manualmente no stream externo
							for _, candidate := range finalResp.Candidates {
								if candidate.Content != nil {
									for _, part := range candidate.Content.Parts {
										if textPart, ok := part.(genai.Text); ok {
											text := string(textPart)
											if callOptions.StreamingFunc != nil {
												callOptions.StreamingFunc(ctx, []byte(text))
											}
											responseChannel <- models.MessageResponse{
												Role:    string(models.TypeAssistantRole),
												Content: text,
											}
										}
									}
								}
							}
							return // Finaliza a goroutine após a resposta final
						}
					}
				}
			}
		}
	}()

	return responseChannel, nil
}

func (c *Client) prepareTools(toolsCall []tools.Tool) ([]*genai.FunctionDeclaration, map[string]tools.Tool) {
	var functionDeclarations []*genai.FunctionDeclaration
	maptools := make(map[string]tools.Tool)

	for _, tool := range toolsCall {
		params, _ := tools.GenerateJSONSchema(tool.GetParameterStruct())
		schema := &genai.Schema{
			Type:       genai.TypeObject,
			Properties: make(map[string]*genai.Schema),
		}

		if propsMap, ok := params["properties"].(map[string]interface{}); ok {
			for propName, propValue := range propsMap {
				if propObj, ok := propValue.(map[string]interface{}); ok {
					schema.Properties[propName] = &genai.Schema{
						Type:        parseSchemaType(propObj["type"].(string)),
						Description: propObj["description"].(string),
					}
					schema.Required = append(schema.Required, propName)
				}
			}
		}

		functionDeclarations = append(functionDeclarations, &genai.FunctionDeclaration{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  schema,
		})

		maptools[tool.Name()] = tool

	}

	return functionDeclarations, maptools
}

func extractToolCall(candidate *genai.Candidate) (tools.ToolCall, bool) {
	for _, part := range candidate.Content.Parts {
		if functionCall, ok := part.(genai.FunctionCall); ok {
			return tools.ToolCall{
				ID: "call_0",
				Function: tools.FunctionCall{
					Name:      functionCall.Name,
					Arguments: string(jsonMarshal(functionCall.Args)),
				},
			}, true
		}
	}
	return tools.ToolCall{}, false
}

func buildCompletionResponse(model string, candidate *genai.Candidate, toolCall *tools.ToolCall) *CompletionResponse {
	toolCalls := []tools.ToolCall{}
	if toolCall != nil {
		toolCalls = append(toolCalls, *toolCall)
	}

	return &CompletionResponse{
		ID:      "",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choices{{
			Index: 0,
			Message: models.MessageResponse{
				Role:      string(models.TypeAssistantRole),
				Content:   extractText(candidate),
				ToolCalls: toolCalls,
			},
			FinishReason: finishReasonToString(candidate.FinishReason),
		}},
	}
}

func ConvertJSONToMap(jsonStr string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("error converting JSON to map: %w", err)
	}
	return result, nil
}

func extractText(candidate *genai.Candidate) string {
	var text string
	for _, part := range candidate.Content.Parts {
		if t, ok := part.(genai.Text); ok {
			text += string(t)
		}
	}
	return text
}

func emptyCompletionResponse(model string) *CompletionResponse {
	return &CompletionResponse{
		ID:      "",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choices{},
	}
}

func jsonMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf("error marshaling JSON: %v", err))
	}
	return data
}

func finishReasonToString(reason genai.FinishReason) string {
	switch reason {
	case genai.FinishReasonStop:
		return "stop"
	case genai.FinishReasonMaxTokens:
		return "length"
	case genai.FinishReasonSafety, genai.FinishReasonRecitation:
		return "content_filter"
	case genai.FinishReasonOther:
		return "other"
	default:
		return "stop"
	}
}

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

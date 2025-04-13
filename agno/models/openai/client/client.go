package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

var baseUrl string = "https://api.openai.com/v1"

// Client represents a customized HTTP client for interacting with the OpenAI API.
type Client struct {
	model   string
	baseURL string
	apiKey  string
	client  *http.Client
	options openai.ClientOptions
}

// NewClient creates a new client for the OpenAI API.
func NewClient(options ...openai.OptionClient) (*Client, error) {
	opts := openai.ClientOptions{}
	for _, option := range options {
		option(&opts)
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("API key not set. Please provide an API key or set the OPENAI_API_KEY environment variable")
		}
		opts.APIKey = apiKey
	}

	if opts.BaseURL == "" {
		opts.BaseURL = baseUrl
	}

	return &Client{
		baseURL: opts.BaseURL,
		model:   opts.ID,
		apiKey:  apiKey,
		client:  http.DefaultClient,
		options: opts,
	}, nil
}

func (c *Client) newRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	var buf io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// Do performs an HTTP request to the OpenAI API.
func (c *Client) Do(ctx context.Context, method, path string, body interface{}, v interface{}) error {
	req, err := c.newRequest(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		errorResponse := struct {
			Error struct {
				Message string      `json:"message"`
				Type    string      `json:"type"`
				Param   string      `json:"param,omitempty"`
				Code    interface{} `json:"code,omitempty"`
			} `json:"error"`
		}{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return fmt.Errorf("API error (%s): %s", errorResponse.Error.Type, errorResponse.Error.Message)
	}

	if v != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(bodyBytes) == 0 {
			return nil
		}
		return json.Unmarshal(bodyBytes, v)
	}
	return nil
}

// CreateChatCompletion creates a chat completion request.
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (*CompletionResponse, error) {
	callOptions := openai.DefaultCallOptions()
	for _, option := range options {
		option(callOptions)
	}

	req := &ChatCompletionRequest{
		Model:               c.model,
		Messages:            messages,
		Store:               callOptions.Store,
		ReasoningEffort:     callOptions.ReasoningEffort,
		Metadata:            callOptions.Metadata,
		FrequencyPenalty:    callOptions.FrequencyPenalty,
		LogitBias:           callOptions.LogitBias,
		Logprobs:            callOptions.Logprobs,
		TopLogprobs:         callOptions.TopLogprobs,
		MaxTokens:           callOptions.MaxTokens,
		MaxCompletionTokens: callOptions.MaxCompletionTokens,
		Modalities:          callOptions.Modalities,
		Audio:               callOptions.Audio,
		PresencePenalty:     callOptions.PresencePenalty,
		ResponseFormat:      callOptions.ResponseFormat,
		Seed:                callOptions.Seed,
		Stop:                callOptions.Stop,
		Temperature:         callOptions.Temperature,
		TopP:                callOptions.TopP,
		Tools:               callOptions.Tools,
		Stream:              callOptions.Stream,
	}

	//create map for tools
	maptools := make(map[string]toolkit.Tool)
	if len(callOptions.ToolCall) > 0 {
		// Set ToolChoice flag as pointer value.
		req.ToolChoice = "auto"
		for _, t := range callOptions.ToolCall {
			for methodName := range t.GetMethods() {
				maptools[methodName] = t
			}

		}
	}

	if callOptions.Stream != nil && *callOptions.Stream {
		// Set Stream flag as pointer value.
		boolTrue := true
		req.Stream = &boolTrue

		// Use newRequest helper to create the HTTP request.
		httpReq, err := c.newRequest(ctx, http.MethodPost, c.baseURL+"/chat/completions", req)
		if err != nil {
			return nil, err
		}

		httpResp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, err
		}
		defer httpResp.Body.Close()

		var completeMessage bytes.Buffer
		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if line == "data: [DONE]" {
				break
			}
			if strings.HasPrefix(line, "data: ") {
				line = strings.TrimPrefix(line, "data: ")
				line = strings.TrimSpace(line)
			}
			if line == "" {
				continue
			}
			var chunk ChatCompletionChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				return nil, err
			}
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content != "" {
					completeMessage.WriteString(delta.Content)
					if callOptions.StreamingFunc != nil {
						if err := callOptions.StreamingFunc(ctx, []byte(delta.Content)); err != nil {
							return nil, err
						}
					}
				}
				if len(delta.ToolCalls) > 0 {
					for _, tc := range delta.ToolCalls {
						if tool, ok := maptools[tc.Function.Name]; ok {
							args, err := json.Marshal(tc.Function.Arguments)
							if err != nil {
								return nil, err
							}

							debug := fmt.Sprintf("Tool %s \n", tc.Function.Name)
							debug += fmt.Sprintf("Args: %s \n", string(args))
							utils.ToolCallPanel(debug)

							resTool, err := tool.Execute(tc.Function.Name, args)
							if err == nil {
								toolResult := resTool.(string)
								completeMessage.WriteString(toolResult)
								if callOptions.StreamingFunc != nil {
									if err := callOptions.StreamingFunc(ctx, []byte(toolResult)); err != nil {
										return nil, err
									}
								}
							}
						}
					}
				}
				if chunk.Choices[0].FinishReason == "stop" {
					break
				}
			}
		}

		// Return a CompletionResponse constructed with the streamed content.
		return &CompletionResponse{
			ID:      "",
			Object:  "chat.completion",
			Created: 0,
			Model:   req.Model,
			Choices: []Choices{
				{
					Message: models.MessageResponse{
						Role:    models.TypeAssistantRole,
						Content: completeMessage.String(),
					},
					Index:        0,
					FinishReason: "stop",
				},
			},
		}, nil
	}

	// If not streaming.
	resp := new(CompletionResponse)
	if err := c.Do(ctx, http.MethodPost, "/chat/completions", req, resp); err != nil {
		return nil, err
	}

	if len(resp.Choices) > 0 {
		req = parserResponseTool(req, resp, maptools)
		if len(maptools) > 0 {
			if err := c.Do(ctx, http.MethodPost, "/chat/completions", req, resp); err != nil {
				return nil, err
			}
		}
	}

	return resp, nil
}

// StreamChatCompletion performs a streaming chat completion request.
func (c *Client) StreamChatCompletion(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan ChatCompletionChunk, error) {

	callOptions := openai.DefaultCallOptions()
	for _, option := range options {
		option(callOptions)
	}

	req := &ChatCompletionRequest{
		Model:               c.model,
		Messages:            messages,
		Store:               callOptions.Store,
		ReasoningEffort:     callOptions.ReasoningEffort,
		Metadata:            callOptions.Metadata,
		FrequencyPenalty:    callOptions.FrequencyPenalty,
		LogitBias:           callOptions.LogitBias,
		Logprobs:            callOptions.Logprobs,
		TopLogprobs:         callOptions.TopLogprobs,
		MaxTokens:           callOptions.MaxTokens,
		MaxCompletionTokens: callOptions.MaxCompletionTokens,
		Modalities:          callOptions.Modalities,
		Audio:               callOptions.Audio,
		PresencePenalty:     callOptions.PresencePenalty,
		ResponseFormat:      callOptions.ResponseFormat,
		Seed:                callOptions.Seed,
		Stop:                callOptions.Stop,
		Temperature:         callOptions.Temperature,
		TopP:                callOptions.TopP,
		Tools:               callOptions.Tools,
		Stream:              callOptions.Stream,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	httpReq.Header.Set("Content-Type", "application/json")
	chunks := make(chan ChatCompletionChunk)
	go func() {
		defer close(chunks)
		resp, err := c.client.Do(httpReq)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk ChatCompletionChunk
			if err := decoder.Decode(&chunk); err == io.EOF {
				break
			} else if err != nil {
				break
			}
			chunks <- chunk
		}
	}()
	return chunks, nil
}

func parserResponseTool(req *ChatCompletionRequest, resp *CompletionResponse, maptools map[string]toolkit.Tool) *ChatCompletionRequest {
	for _, choice := range resp.Choices {
		if choice.Message.Role == models.TypeAssistantRole {
			//add message assistente
			req.Messages = append(req.Messages, models.Message{
				Role:      models.TypeAssistantRole,
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})
			if len(choice.Message.ToolCalls) > 0 {
				for _, tc := range choice.Message.ToolCalls {
					//check if the function exists in the map
					if tcm, ok := maptools[tc.Function.Name]; ok {
						args, err := json.Marshal(tc.Function.Arguments)
						if err != nil {
							return nil
						}
						debug := fmt.Sprintf("Tool %s \n", tc.Function.Name)
						debug += fmt.Sprintf("Args: %s \n", string(args))
						utils.ToolCallPanel(debug)
						//execute the tool
						resTool, err := tcm.Execute(tc.Function.Name, args)
						if err != nil {
							return nil
						}
						//update the response content
						req.Messages = append(req.Messages, models.Message{
							ToolCallID: &tc.ID,
							Role:       models.TypeToolRole,
							Content:    resTool.(string),
						})
					}
				}
			}
		}
	}

	return req
}

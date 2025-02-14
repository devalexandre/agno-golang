package openai

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
)

var baseUrl string = "https://api.openai.com/v1"

// Option defines a function that modifies the client options.
type OptionClient func(*ClientOptions)

// Client represents a customized HTTP client for interacting with the OpenAI API.
type Client struct {
	model   string
	baseURL string
	apiKey  string
	client  *http.Client
	options ClientOptions
}

// NewClient creates a new client for the OpenAI API.
func NewClient(options ...OptionClient) (*Client, error) {
	opts := ClientOptions{}
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
		model:   opts.Model,
		apiKey:  apiKey,
		client:  http.DefaultClient,
		options: opts,
	}, nil
}

// Do performs an HTTP request to the OpenAI API.
func (c *Client) Do(ctx context.Context, method, path string, body interface{}, v interface{}) error {
	var buf io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		errorResponse := struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Param   string `json:"param,omitempty"`
				Code    string `json:"code,omitempty"`
			} `json:"error"`
		}{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return fmt.Errorf("API error (%s): %s", errorResponse.Error.Type, errorResponse.Error.Message)
	}

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

// CreateChatCompletion creates a chat completion request.
func (c *Client) CreateChatCompletion(ctx context.Context, messages []models.Message, options ...Option) (*CompletionResponse, error) {
	callOptions := DefaultCallOptions()
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

	if callOptions.Stream != nil && *callOptions.Stream {
		// Set Stream flag as pointer value.
		boolTrue := true
		req.Stream = &boolTrue

		// Marshal the request body.
		jsonBody, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		// Create a new HTTP request.
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
		httpReq.Header.Set("Content-Type", "application/json")

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
					Message: models.Message{
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
	resp := &CompletionResponse{}
	if err := c.Do(ctx, http.MethodPost, "/chat/completions", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

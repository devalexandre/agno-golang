// Package ibm provides a native client for IBM watsonx.ai chat models
// (POST {url}/ml/v1/text/chat), mirroring Agno's WatsonX model.
//
// The API key is exchanged for an IAM bearer token (cached until shortly
// before expiry). Requests carry model_id and project_id; tool calls use the
// OpenAI-style function schema and are executed by the client (as the other
// agno-golang providers do). Streaming uses /ml/v1/text/chat_stream (SSE).
package ibm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const (
	// DefaultURL is the watsonx.ai regional endpoint used by Agno.
	DefaultURL = "https://eu-de.ml.cloud.ibm.com"
	// DefaultIAMURL exchanges the API key for a bearer token.
	DefaultIAMURL = "https://iam.cloud.ibm.com/identity/token"
	// DefaultAPIVersion is the watsonx.ai "version" query parameter.
	DefaultAPIVersion = "2024-10-08"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "ibm/granite-20b-code-instruct"

	EnvAPIKey    = "IBM_WATSONX_API_KEY"
	EnvProjectID = "IBM_WATSONX_PROJECT_ID"
	EnvURL       = "IBM_WATSONX_URL"

	maxToolRounds = 20
)

// Config configures a watsonx.ai chat model.
type Config struct {
	ID         string
	APIKey     string // IBM Cloud API key (IBM_WATSONX_API_KEY)
	ProjectID  string // IBM_WATSONX_PROJECT_ID
	URL        string // IBM_WATSONX_URL, default DefaultURL
	IAMURL     string // default DefaultIAMURL
	APIVersion string
	// BearerToken skips the IAM exchange (e.g. a pre-issued token).
	BearerToken string
	HTTPClient  *http.Client

	MaxTokens        *int
	Temperature      *float64
	TopP             *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	RequestParams    map[string]interface{}
}

// WatsonX is an IBM watsonx.ai chat model.
type WatsonX struct {
	cfg  Config
	http *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// New builds a WatsonX model from a Config, applying env fallbacks.
func New(cfg Config) (*WatsonX, error) {
	if cfg.ID == "" {
		cfg.ID = DefaultModel
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv(EnvAPIKey)
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = os.Getenv(EnvProjectID)
	}
	if env := os.Getenv(EnvURL); env != "" && cfg.URL == "" {
		cfg.URL = env
	}
	if cfg.URL == "" {
		cfg.URL = DefaultURL
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	if cfg.IAMURL == "" {
		cfg.IAMURL = DefaultIAMURL
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	if cfg.APIKey == "" && cfg.BearerToken == "" {
		return nil, fmt.Errorf("ibm: %s not set", EnvAPIKey)
	}
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("ibm: %s not set", EnvProjectID)
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return &WatsonX{cfg: cfg, http: hc}, nil
}

// NewWatsonXChat builds a WatsonX model from client options: ID, APIKey,
// BaseURL (the watsonx URL), MaxTokens/Temperature/TopP and ClientParams
// "project_id", "iam_url", "api_version", "bearer_token".
func NewWatsonXChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := &models.ClientOptions{}
	for _, o := range options {
		o(co)
	}
	str := func(k string) string { s, _ := co.ClientParams[k].(string); return s }
	cfg := Config{
		ID:          co.ID,
		APIKey:      co.APIKey,
		URL:         co.BaseURL,
		ProjectID:   str("project_id"),
		IAMURL:      str("iam_url"),
		APIVersion:  str("api_version"),
		BearerToken: str("bearer_token"),
		HTTPClient:  co.HTTPClient,
		MaxTokens:   co.MaxTokens,
	}
	if co.Temperature != nil {
		v := float64(*co.Temperature)
		cfg.Temperature = &v
	}
	if co.TopP != nil {
		v := float64(*co.TopP)
		cfg.TopP = &v
	}
	return New(cfg)
}

// GetID returns the model ID.
func (w *WatsonX) GetID() string { return w.cfg.ID }

// bearer returns a valid IAM token, exchanging the API key when needed.
func (w *WatsonX) bearer(ctx context.Context) (string, error) {
	if w.cfg.BearerToken != "" {
		return w.cfg.BearerToken, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.token != "" && time.Now().Before(w.expires) {
		return w.token, nil
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	form.Set("apikey", w.cfg.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.IAMURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ibm: IAM token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return "", fmt.Errorf("ibm: IAM token request: %d %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		return "", errors.New("ibm: invalid IAM token response")
	}
	w.token = tok.AccessToken
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	w.expires = time.Now().Add(ttl - time.Minute)
	return w.token, nil
}

type chatMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"`
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type chatResponse struct {
	ModelID string `json:"model_id"`
	Choices []struct {
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []tools.ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func formatMessages(messages []models.Message) []chatMessage {
	out := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		cm := chatMessage{Role: string(m.Role), Content: m.Content}
		switch m.Role {
		case models.TypeAssistantRole:
			cm.ToolCalls = m.ToolCalls
			if m.Content == "" {
				cm.Content = nil
			}
		case models.TypeToolRole:
			if m.ToolCallID != nil {
				cm.ToolCallID = *m.ToolCallID
			}
		}
		out = append(out, cm)
	}
	return out
}

func (w *WatsonX) buildBody(call *models.CallOptions, msgs []chatMessage, toolDefs []interface{}) map[string]interface{} {
	body := map[string]interface{}{
		"model_id":   w.cfg.ID,
		"project_id": w.cfg.ProjectID,
		"messages":   msgs,
	}
	setF := func(k string, cfgV *float64, callV *float32) {
		if callV != nil {
			body[k] = float64(*callV)
		} else if cfgV != nil {
			body[k] = *cfgV
		}
	}
	setF("temperature", w.cfg.Temperature, call.Temperature)
	setF("top_p", w.cfg.TopP, call.TopP)
	setF("frequency_penalty", w.cfg.FrequencyPenalty, call.FrequencyPenalty)
	setF("presence_penalty", w.cfg.PresencePenalty, call.PresencePenalty)
	if call.MaxTokens != nil {
		body["max_tokens"] = *call.MaxTokens
	} else if w.cfg.MaxTokens != nil {
		body["max_tokens"] = *w.cfg.MaxTokens
	}
	if call.ResponseFormat != nil {
		if s, ok := call.ResponseFormat.(string); ok {
			body["response_format"] = map[string]interface{}{"type": s}
		} else {
			body["response_format"] = call.ResponseFormat
		}
	}
	if len(toolDefs) > 0 {
		body["tools"] = toolDefs
	}
	for k, v := range w.cfg.RequestParams {
		body[k] = v
	}
	for k, v := range call.RequestParams {
		body[k] = v
	}
	return body
}

func (w *WatsonX) post(ctx context.Context, path string, body map[string]interface{}, stream bool) (*http.Response, error) {
	token, err := w.bearer(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	u := w.cfg.URL + path + "?version=" + url.QueryEscape(w.cfg.APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("ibm: POST %s: %d %s", path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}

// turn performs one request and returns content and tool calls.
func (w *WatsonX) turn(ctx context.Context, call *models.CallOptions, body map[string]interface{}) (string, []tools.ToolCall, error) {
	if call.StreamingFunc == nil {
		resp, err := w.post(ctx, "/ml/v1/text/chat", body, false)
		if err != nil {
			return "", nil, err
		}
		defer resp.Body.Close()
		var cr chatResponse
		if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
			return "", nil, fmt.Errorf("ibm: decode response: %w", err)
		}
		if len(cr.Choices) == 0 {
			return "", nil, errors.New("ibm: no choices in response")
		}
		return cr.Choices[0].Message.Content, cr.Choices[0].Message.ToolCalls, nil
	}

	resp, err := w.post(ctx, "/ml/v1/text/chat_stream", body, true)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	var content strings.Builder
	calls := map[int]*tools.ToolCall{}
	var order []int
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		if d.Content != "" {
			content.WriteString(d.Content)
			if err := call.StreamingFunc(ctx, []byte(d.Content)); err != nil {
				return "", nil, err
			}
		}
		for _, tc := range d.ToolCalls {
			cur, ok := calls[tc.Index]
			if !ok {
				cur = &tools.ToolCall{Type: "function"}
				calls[tc.Index] = cur
				order = append(order, tc.Index)
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			cur.Function.Name += tc.Function.Name
			cur.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	var out []tools.ToolCall
	for _, i := range order {
		out = append(out, *calls[i])
	}
	return content.String(), out, nil
}

func buildTools(toolkits []toolkit.Tool) ([]interface{}, map[string]toolkit.Tool) {
	var defs []interface{}
	byName := map[string]toolkit.Tool{}
	for _, tk := range toolkits {
		for method := range tk.GetMethods() {
			desc := tk.GetDescriptionOfMethod(method)
			if desc == "" {
				desc = tk.GetDescription()
			}
			defs = append(defs, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        method,
					"description": desc,
					"parameters":  tk.GetParameterStruct(method),
				},
			})
			byName[method] = tk
		}
	}
	return defs, byName
}

func (w *WatsonX) run(ctx context.Context, messages []models.Message, options []models.Option) (*models.MessageResponse, error) {
	call := &models.CallOptions{}
	for _, o := range options {
		o(call)
	}
	toolDefs, byName := buildTools(call.ToolCall)
	msgs := formatMessages(messages)
	var allCalls []tools.ToolCall
	var results []models.ToolResult

	for round := 0; ; round++ {
		content, toolCalls, err := w.turn(ctx, call, w.buildBody(call, msgs, toolDefs))
		if err != nil {
			return nil, err
		}
		if len(toolCalls) == 0 || len(byName) == 0 {
			return &models.MessageResponse{
				Model:       w.cfg.ID,
				Role:        models.TypeAssistantRole,
				Content:     content,
				ToolCalls:   append(allCalls, toolCalls...),
				ToolResults: results,
			}, nil
		}
		if round+1 >= maxToolRounds {
			return nil, fmt.Errorf("ibm: exceeded %d tool rounds", maxToolRounds)
		}
		allCalls = append(allCalls, toolCalls...)
		assistant := chatMessage{Role: "assistant", ToolCalls: toolCalls}
		if content != "" {
			assistant.Content = content
		}
		msgs = append(msgs, assistant)
		for _, tc := range toolCalls {
			out, errStr := execute(byName, tc)
			results = append(results, models.ToolResult{ToolName: tc.Function.Name, ToolInput: tc.Function.Arguments, Result: out, Error: errStr})
			msgs = append(msgs, chatMessage{Role: "tool", Content: out, ToolCallID: tc.ID})
		}
	}
}

func execute(byName map[string]toolkit.Tool, tc tools.ToolCall) (string, string) {
	tk, ok := byName[tc.Function.Name]
	if !ok {
		msg := fmt.Sprintf("Tool %s not found", tc.Function.Name)
		return msg, msg
	}
	args := tc.Function.Arguments
	if strings.TrimSpace(args) == "" {
		args = "{}"
	}
	res, err := tk.Execute(tc.Function.Name, json.RawMessage(args))
	if err != nil {
		return fmt.Sprintf("Error executing tool: %v", err), err.Error()
	}
	if s, ok := res.(string); ok {
		return s, ""
	}
	raw, _ := json.Marshal(res)
	return string(raw), ""
}

// Invoke sends a chat request (running the tool loop) and returns the final turn.
func (w *WatsonX) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	return w.run(ctx, messages, options)
}

// AInvoke runs Invoke in a goroutine.
func (w *WatsonX) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		resp, err := w.run(ctx, messages, options)
		if err != nil {
			errCh <- err
			return
		}
		ch <- resp
	}()
	return ch, errCh
}

// InvokeStream streams content deltas to the models.WithStreamingFunc callback.
func (w *WatsonX) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range options {
		o(call)
	}
	if call.StreamingFunc == nil {
		return errors.New("ibm: streaming function required for InvokeStream")
	}
	_, err := w.run(ctx, messages, options)
	return err
}

// AInvokeStream streams content deltas as MessageResponse chunks on a channel.
func (w *WatsonX) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 16)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		opts := append(append([]models.Option{}, options...), models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			select {
			case ch <- &models.MessageResponse{Role: models.TypeAssistantRole, Content: string(chunk)}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
		if _, err := w.run(ctx, messages, opts); err != nil {
			errCh <- err
		}
	}()
	return ch, errCh
}

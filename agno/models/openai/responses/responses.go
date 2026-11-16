// Package responses implements models.AgnoModelInterface over the OpenAI
// Responses API (POST /v1/responses), mirroring Agno's OpenAIResponses.
//
// It converts messages to Responses input items (system -> developer, tool
// results -> function_call_output), sends function tools, runs the tool loop
// itself (like the chat client), chains tool rounds with previous_response_id
// when responses are stored, and otherwise replays reasoning items (requesting
// reasoning.encrypted_content for reasoning models). Streaming uses the SSE
// event stream (response.output_text.delta, response.completed, ...).
//
// The same client serves "Open Responses" compatible gateways (NewOpenResponses)
// and Azure OpenAI (agno/models/azure.NewAzureResponses).
package responses

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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
	// DefaultBaseURL is the OpenAI API base URL.
	DefaultBaseURL = "https://api.openai.com/v1"
	// EnvAPIKey is read when no API key is provided.
	EnvAPIKey = "OPENAI_API_KEY"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gpt-5.4-mini"
	// DefaultMaxToolRounds bounds the internal tool-call loop.
	DefaultMaxToolRounds = 20
)

// Config configures a Responses API client.
type Config struct {
	ID           string
	APIKey       string // sent as "Authorization: Bearer"; empty sends no Authorization header
	BaseURL      string // the client POSTs to BaseURL + "/responses"
	Organization string
	Headers      http.Header       // default headers (e.g. Azure "api-key")
	Query        map[string]string // default query params (e.g. Azure "api-version")
	HTTPClient   *http.Client
	Timeout      time.Duration

	// Request parameters (nil/empty values are omitted).
	Store             *bool
	ReasoningEffort   string
	ReasoningSummary  string
	Reasoning         map[string]interface{}
	Include           []string
	MaxOutputTokens   *int
	MaxToolCalls      *int
	Temperature       *float64
	TopP              *float64
	ParallelToolCalls *bool
	Truncation        string
	User              string
	ServiceTier       string
	Verbosity         string
	Metadata          map[string]interface{}
	ExtraBody         map[string]interface{}

	// PreviousResponseID continues a stored conversation on the first request.
	PreviousResponseID string
	// DisablePreviousResponseID replays the full context instead of chaining
	// tool rounds through previous_response_id.
	DisablePreviousResponseID bool
	// IsReasoningModel overrides the id-prefix detection (o3, o4-mini, gpt-5, gpt-6).
	IsReasoningModel *bool
	// MaxToolRounds bounds the tool loop (DefaultMaxToolRounds when 0).
	MaxToolRounds int
}

// Responses is a Responses API model.
type Responses struct {
	cfg  Config
	http *http.Client

	mu             sync.Mutex
	lastResponseID string
}

// New builds a Responses model from a Config. ID, APIKey and BaseURL default to
// DefaultModel, $OPENAI_API_KEY and DefaultBaseURL.
func New(cfg Config) (*Responses, error) {
	if cfg.ID == "" {
		cfg.ID = DefaultModel
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.MaxToolRounds <= 0 {
		cfg.MaxToolRounds = DefaultMaxToolRounds
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: cfg.Timeout}
	}
	return &Responses{cfg: cfg, http: hc}, nil
}

// NewOpenAIResponses builds an OpenAI Responses model from client options.
// Besides ID/APIKey/BaseURL/DefaultHeaders/DefaultQuery/HTTPClient/MaxTokens/
// Temperature/TopP, these ClientParams keys are honoured: store (bool),
// reasoning_effort, reasoning_summary, include ([]string), previous_response_id,
// use_previous_response_id (bool), is_reasoning_model (bool), verbosity,
// truncation, user, service_tier, parallel_tool_calls (bool), max_tool_calls
// (int), metadata (map), extra_body (map).
func NewOpenAIResponses(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := &models.ClientOptions{}
	for _, o := range options {
		o(co)
	}
	cfg := ConfigFromClientOptions(co)
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv(EnvAPIKey)
	}
	if cfg.APIKey == "" && cfg.BaseURL == "" {
		return nil, fmt.Errorf("responses: API key not set; provide one or set %s", EnvAPIKey)
	}
	return New(cfg)
}

// NewOpenResponses builds a client for providers implementing the Open
// Responses specification (https://openresponses.org): stateless by default
// (store=false) and without reasoning-model special handling.
func NewOpenResponses(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	co := &models.ClientOptions{}
	for _, o := range options {
		o(co)
	}
	cfg := ConfigFromClientOptions(co)
	if cfg.Store == nil {
		f := false
		cfg.Store = &f
	}
	if cfg.IsReasoningModel == nil {
		f := false
		cfg.IsReasoningModel = &f
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "not-provided"
	}
	return New(cfg)
}

// ConfigFromClientOptions maps generic client options (see NewOpenAIResponses)
// to a Config. It does not apply environment fallbacks.
func ConfigFromClientOptions(co *models.ClientOptions) Config {
	cfg := Config{
		ID:           co.ID,
		APIKey:       co.APIKey,
		BaseURL:      co.BaseURL,
		Organization: co.Organization,
		Headers:      co.DefaultHeaders,
		Query:        co.DefaultQuery,
		HTTPClient:   co.HTTPClient,
	}
	if co.Timeout > 0 {
		cfg.Timeout = time.Duration(co.Timeout) * time.Second
	}
	if co.MaxTokens != nil {
		v := *co.MaxTokens
		cfg.MaxOutputTokens = &v
	}
	if co.Temperature != nil {
		v := float64(*co.Temperature)
		cfg.Temperature = &v
	}
	if co.TopP != nil {
		v := float64(*co.TopP)
		cfg.TopP = &v
	}
	p := co.ClientParams
	str := func(k string) string { s, _ := p[k].(string); return s }
	if v, ok := p["store"].(bool); ok {
		cfg.Store = &v
	}
	if v, ok := p["is_reasoning_model"].(bool); ok {
		cfg.IsReasoningModel = &v
	}
	if v, ok := p["parallel_tool_calls"].(bool); ok {
		cfg.ParallelToolCalls = &v
	}
	if v, ok := p["use_previous_response_id"].(bool); ok {
		cfg.DisablePreviousResponseID = !v
	}
	if v, ok := p["max_tool_calls"].(int); ok {
		cfg.MaxToolCalls = &v
	}
	switch v := p["include"].(type) {
	case []string:
		cfg.Include = v
	case []interface{}:
		for _, s := range v {
			if s, ok := s.(string); ok {
				cfg.Include = append(cfg.Include, s)
			}
		}
	}
	if v, ok := p["metadata"].(map[string]interface{}); ok {
		cfg.Metadata = v
	}
	if v, ok := p["extra_body"].(map[string]interface{}); ok {
		cfg.ExtraBody = v
	}
	if v, ok := p["reasoning"].(map[string]interface{}); ok {
		cfg.Reasoning = v
	}
	cfg.ReasoningEffort = str("reasoning_effort")
	cfg.ReasoningSummary = str("reasoning_summary")
	cfg.PreviousResponseID = str("previous_response_id")
	cfg.Verbosity = str("verbosity")
	cfg.Truncation = str("truncation")
	cfg.User = str("user")
	cfg.ServiceTier = str("service_tier")
	return cfg
}

// GetID returns the model ID.
func (r *Responses) GetID() string { return r.cfg.ID }

// Config returns a copy of the client configuration.
func (r *Responses) Config() Config { return r.cfg }

// LastResponseID returns the id of the last response received. Pass it as
// Config.PreviousResponseID (or the "previous_response_id" request param) to
// continue a stored conversation.
func (r *Responses) LastResponseID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastResponseID
}

// IsReasoningModel reports whether the model is treated as a reasoning model.
func (r *Responses) IsReasoningModel() bool {
	if r.cfg.IsReasoningModel != nil {
		return *r.cfg.IsReasoningModel
	}
	for _, p := range []string{"o3", "o4-mini", "gpt-5", "gpt-6"} {
		if strings.HasPrefix(r.cfg.ID, p) {
			return true
		}
	}
	return false
}

// chains reports whether tool rounds continue through previous_response_id.
func (r *Responses) chains(store *bool) bool {
	return !r.cfg.DisablePreviousResponseID && (store == nil || *store)
}

// ---------------------------------------------------------------------------
// Wire types

// apiResponse is the subset of the Responses object the client reads.
type apiResponse struct {
	ID     string            `json:"id"`
	Model  string            `json:"model"`
	Status string            `json:"status"`
	Output []json.RawMessage `json:"output"`
	Error  *apiError         `json:"error"`
	Usage  json.RawMessage   `json:"usage,omitempty"`

	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

type outputItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Role      string `json:"role"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal string `json:"refusal"`
	} `json:"content"`
	Summary []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"summary"`
	EncryptedContent string `json:"encrypted_content"`
}

// parsed is the model turn extracted from a response.
type parsed struct {
	id        string
	model     string
	content   string
	reasoning string
	toolCalls []tools.ToolCall
	// replay holds the reasoning and function_call items to send back when the
	// context is replayed instead of chained.
	replay []interface{}
}

func parseResponse(resp *apiResponse) (*parsed, error) {
	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("responses: %s (%s)", resp.Error.Message, resp.Error.Type)
	}
	if resp.Status == "failed" {
		return nil, errors.New("responses: response failed")
	}
	out := &parsed{id: resp.ID, model: resp.Model}
	var content, reasoning strings.Builder
	for _, raw := range resp.Output {
		var item outputItem
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				switch c.Type {
				case "output_text":
					content.WriteString(c.Text)
				case "refusal":
					content.WriteString(c.Refusal)
				}
			}
		case "function_call":
			out.toolCalls = append(out.toolCalls, tools.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: tools.FunctionCall{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
			out.replay = append(out.replay, map[string]interface{}{
				"type":      "function_call",
				"call_id":   item.CallID,
				"name":      item.Name,
				"arguments": item.Arguments,
			})
		case "reasoning":
			for _, s := range item.Summary {
				if reasoning.Len() > 0 {
					reasoning.WriteString("\n")
				}
				reasoning.WriteString(s.Text)
			}
			out.replay = append(out.replay, json.RawMessage(raw))
		}
	}
	out.content = content.String()
	out.reasoning = reasoning.String()
	return out, nil
}

// ---------------------------------------------------------------------------
// Request building

// formatInput converts agno messages to Responses input items.
func formatInput(messages []models.Message) []interface{} {
	items := make([]interface{}, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case models.TypeSystemRole:
			items = append(items, map[string]interface{}{"role": "developer", "content": msg.Content})
		case models.TypeUserRole:
			items = append(items, map[string]interface{}{"role": "user", "content": userContent(msg)})
		case models.TypeAssistantRole:
			if msg.Content != "" {
				items = append(items, map[string]interface{}{"role": "assistant", "content": msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				items = append(items, map[string]interface{}{
					"type":      "function_call",
					"call_id":   tc.ID,
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				})
			}
		case models.TypeToolRole:
			if msg.ToolCallID != nil {
				items = append(items, map[string]interface{}{
					"type":    "function_call_output",
					"call_id": *msg.ToolCallID,
					"output":  msg.Content,
				})
			}
		default:
			items = append(items, map[string]interface{}{"role": string(msg.Role), "content": msg.Content})
		}
	}
	return items
}

func userContent(msg models.Message) interface{} {
	if len(msg.Images) == 0 && len(msg.Files) == 0 {
		return msg.Content
	}
	parts := []interface{}{}
	if msg.Content != "" {
		parts = append(parts, map[string]interface{}{"type": "input_text", "text": msg.Content})
	}
	for _, img := range msg.Images {
		part := map[string]interface{}{"type": "input_image"}
		switch {
		case img.ID != "":
			part["file_id"] = img.ID
		case img.URL != "":
			part["image_url"] = img.URL
		case len(img.Data) > 0:
			mime := img.MimeType
			if mime == "" {
				mime = http.DetectContentType(img.Data)
			}
			part["image_url"] = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
		default:
			continue
		}
		detail := img.Detail
		if detail == "" {
			detail = "auto"
		}
		part["detail"] = detail
		parts = append(parts, part)
	}
	for _, f := range msg.Files {
		part := map[string]interface{}{"type": "input_file"}
		switch {
		case f.ID != "":
			part["file_id"] = f.ID
		case f.URL != "":
			part["file_url"] = f.URL
		case len(f.Data) > 0:
			mime := f.MimeType
			if mime == "" {
				mime = "application/pdf"
			}
			name := f.Name
			if name == "" {
				name = "file"
			}
			part["filename"] = name
			part["file_data"] = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(f.Data)
		default:
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func buildTools(toolkits []toolkit.Tool) ([]interface{}, map[string]toolkit.Tool) {
	var out []interface{}
	byName := map[string]toolkit.Tool{}
	for _, tk := range toolkits {
		for method := range tk.GetMethods() {
			desc := tk.GetDescriptionOfMethod(method)
			if desc == "" {
				desc = tk.GetDescription()
			}
			out = append(out, map[string]interface{}{
				"type":        "function",
				"name":        method,
				"description": desc,
				"parameters":  tk.GetParameterStruct(method),
			})
			byName[method] = tk
		}
	}
	return out, byName
}

// textFormat maps a chat-style response_format to the Responses text.format.
func textFormat(rf interface{}) interface{} {
	switch v := rf.(type) {
	case string:
		return map[string]interface{}{"type": v}
	case map[string]string:
		out := map[string]interface{}{}
		for k, val := range v {
			out[k] = val
		}
		return out
	case map[string]interface{}:
		if v["type"] == "json_schema" {
			if js, ok := v["json_schema"].(map[string]interface{}); ok {
				name, _ := js["name"].(string)
				if name == "" {
					name = "output"
				}
				strict, _ := js["strict"].(bool)
				schema := js["schema"]
				if schema == nil {
					schema = map[string]interface{}{}
				}
				return map[string]interface{}{"type": "json_schema", "name": name, "schema": schema, "strict": strict}
			}
		}
		return v
	}
	return rf
}

// buildRequest assembles the request body for one round.
func (r *Responses) buildRequest(call *models.CallOptions, input []interface{}, toolDefs []interface{}, previousID string, store *bool) map[string]interface{} {
	cfg := r.cfg
	body := map[string]interface{}{
		"model": cfg.ID,
		"input": input,
	}
	if store != nil {
		body["store"] = *store
	}
	if v := cfg.MaxOutputTokens; v != nil {
		body["max_output_tokens"] = *v
	}
	if call.MaxCompletionTokens != nil {
		body["max_output_tokens"] = *call.MaxCompletionTokens
	} else if call.MaxTokens != nil {
		body["max_output_tokens"] = *call.MaxTokens
	}
	if cfg.MaxToolCalls != nil {
		body["max_tool_calls"] = *cfg.MaxToolCalls
	}
	if cfg.Temperature != nil {
		body["temperature"] = *cfg.Temperature
	}
	if call.Temperature != nil {
		body["temperature"] = float64(*call.Temperature)
	}
	if cfg.TopP != nil {
		body["top_p"] = *cfg.TopP
	}
	if call.TopP != nil {
		body["top_p"] = float64(*call.TopP)
	}
	if cfg.ParallelToolCalls != nil {
		body["parallel_tool_calls"] = *cfg.ParallelToolCalls
	}
	if cfg.Truncation != "" {
		body["truncation"] = cfg.Truncation
	}
	if cfg.User != "" {
		body["user"] = cfg.User
	}
	if cfg.ServiceTier != "" {
		body["service_tier"] = cfg.ServiceTier
	}
	metadata := map[string]interface{}{}
	for k, v := range cfg.Metadata {
		metadata[k] = v
	}
	for k, v := range call.Metadata {
		metadata[k] = v
	}
	if len(metadata) > 0 {
		body["metadata"] = metadata
	}

	reasoning := map[string]interface{}{}
	for k, v := range cfg.Reasoning {
		reasoning[k] = v
	}
	if cfg.ReasoningEffort != "" {
		reasoning["effort"] = cfg.ReasoningEffort
	}
	if call.ReasoningEffort != nil && *call.ReasoningEffort != "" {
		reasoning["effort"] = *call.ReasoningEffort
	}
	if cfg.ReasoningSummary != "" {
		reasoning["summary"] = cfg.ReasoningSummary
	}
	if len(reasoning) > 0 {
		body["reasoning"] = reasoning
	}

	text := map[string]interface{}{}
	if cfg.Verbosity != "" {
		text["verbosity"] = cfg.Verbosity
	}
	if call.ResponseFormat != nil {
		text["format"] = textFormat(call.ResponseFormat)
	}
	if len(text) > 0 {
		body["text"] = text
	}

	include := append([]string(nil), cfg.Include...)
	if r.IsReasoningModel() && !r.chains(store) {
		if !containsString(include, "reasoning.encrypted_content") {
			include = append(include, "reasoning.encrypted_content")
		}
	}
	if len(include) > 0 {
		body["include"] = include
	}

	if len(toolDefs) > 0 {
		body["tools"] = toolDefs
		body["tool_choice"] = "auto"
	}
	if previousID != "" {
		body["previous_response_id"] = previousID
	}
	// ExtraBody entries override the body; a nil value removes the field.
	for k, v := range cfg.ExtraBody {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	for k, v := range call.RequestParams {
		body[k] = v
	}
	return body
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// HTTP

func (r *Responses) newRequest(ctx context.Context, call *models.CallOptions, body map[string]interface{}) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("responses: encode request: %w", err)
	}
	u, err := url.Parse(r.cfg.BaseURL + "/responses")
	if err != nil {
		return nil, fmt.Errorf("responses: invalid base URL: %w", err)
	}
	q := u.Query()
	for k, v := range r.cfg.Query {
		q.Set(k, v)
	}
	for k, v := range call.ExtraQuery {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	}
	if r.cfg.Organization != "" {
		req.Header.Set("OpenAI-Organization", r.cfg.Organization)
	}
	for k, vs := range r.cfg.Headers {
		req.Header.Del(k)
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, vs := range call.ExtraHeaders {
		req.Header.Del(k)
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if body["stream"] == true {
		req.Header.Set("Accept", "text/event-stream")
	}
	return req, nil
}

func httpError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var wrapped struct {
		Error *apiError `json:"error"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Error != nil && wrapped.Error.Message != "" {
		return fmt.Errorf("responses: %s %s: %d %s", resp.Request.Method, resp.Request.URL.Redacted(), resp.StatusCode, wrapped.Error.Message)
	}
	return fmt.Errorf("responses: %s %s: %d %s", resp.Request.Method, resp.Request.URL.Redacted(), resp.StatusCode, strings.TrimSpace(string(raw)))
}

// send performs one non-streaming request.
func (r *Responses) send(ctx context.Context, call *models.CallOptions, body map[string]interface{}) (*apiResponse, error) {
	req, err := r.newRequest(ctx, call, body)
	if err != nil {
		return nil, err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, httpError(resp)
	}
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("responses: decode response: %w", err)
	}
	return &out, nil
}

// sendStream performs one streaming request, forwarding text deltas to the
// streaming callback, and returns the final response object.
func (r *Responses) sendStream(ctx context.Context, call *models.CallOptions, body map[string]interface{}) (*apiResponse, error) {
	body["stream"] = true
	req, err := r.newRequest(ctx, call, body)
	if err != nil {
		return nil, err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, httpError(resp)
	}

	var final *apiResponse
	var items []json.RawMessage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		payload := data.String()
		data.Reset()
		if payload == "[DONE]" {
			return nil
		}
		var ev struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Item     json.RawMessage `json:"item"`
			Response *apiResponse    `json:"response"`
			Message  string          `json:"message"`
			Code     any             `json:"code"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			return nil
		}
		switch ev.Type {
		case "response.output_text.delta", "response.refusal.delta":
			if call.StreamingFunc != nil && ev.Delta != "" {
				if err := call.StreamingFunc(ctx, []byte(ev.Delta)); err != nil {
					return err
				}
			}
		case "response.output_item.done":
			if len(ev.Item) > 0 {
				items = append(items, ev.Item)
			}
		case "response.completed", "response.incomplete":
			final = ev.Response
		case "response.failed":
			if ev.Response != nil && ev.Response.Error != nil {
				return fmt.Errorf("responses: %s", ev.Response.Error.Message)
			}
			return errors.New("responses: response failed")
		case "error":
			return fmt.Errorf("responses: stream error: %s", ev.Message)
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return nil, err
			}
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteString("\n")
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if final == nil {
		final = &apiResponse{Status: "completed"}
	}
	if len(final.Output) == 0 {
		final.Output = items
	}
	return final, nil
}

// ---------------------------------------------------------------------------
// Tool loop

func (r *Responses) run(ctx context.Context, messages []models.Message, options []models.Option) (*models.MessageResponse, error) {
	call := &models.CallOptions{}
	for _, o := range options {
		o(call)
	}
	stream := call.StreamingFunc != nil

	store := r.cfg.Store
	if call.Store != nil {
		s := *call.Store
		store = &s
	}
	// Reasoning models need the response stored to chain, or encrypted
	// reasoning replayed when not (mirrors Agno).
	if r.IsReasoningModel() && store == nil {
		t := true
		store = &t
	}

	toolDefs, byName := buildTools(call.ToolCall)

	input := formatInput(messages)
	previousID := r.cfg.PreviousResponseID
	if v, ok := call.RequestParams["previous_response_id"].(string); ok {
		previousID = v
	}

	var allToolCalls []tools.ToolCall
	var toolResults []models.ToolResult
	var reasoning []string

	for round := 0; ; round++ {
		body := r.buildRequest(call, input, toolDefs, previousID, store)

		var resp *apiResponse
		var err error
		if stream {
			resp, err = r.sendStream(ctx, call, body)
		} else {
			resp, err = r.send(ctx, call, body)
		}
		if err != nil {
			return nil, err
		}
		turn, err := parseResponse(resp)
		if err != nil {
			return nil, err
		}
		if turn.id != "" {
			r.mu.Lock()
			r.lastResponseID = turn.id
			r.mu.Unlock()
		}
		if turn.reasoning != "" {
			reasoning = append(reasoning, turn.reasoning)
		}

		if len(turn.toolCalls) == 0 || len(byName) == 0 {
			model := turn.model
			if model == "" {
				model = r.cfg.ID
			}
			return &models.MessageResponse{
				Model:            model,
				Role:             models.TypeAssistantRole,
				Content:          turn.content,
				ToolCalls:        append(allToolCalls, turn.toolCalls...),
				ToolResults:      toolResults,
				ReasoningContent: strings.Join(reasoning, "\n"),
			}, nil
		}
		if round+1 >= r.cfg.MaxToolRounds {
			return nil, fmt.Errorf("responses: exceeded %d tool rounds", r.cfg.MaxToolRounds)
		}

		allToolCalls = append(allToolCalls, turn.toolCalls...)
		outputs := make([]interface{}, 0, len(turn.toolCalls))
		for _, tc := range turn.toolCalls {
			result, errStr := executeTool(byName, tc)
			toolResults = append(toolResults, models.ToolResult{
				ToolName:  tc.Function.Name,
				ToolInput: tc.Function.Arguments,
				Result:    result,
				Error:     errStr,
			})
			outputs = append(outputs, map[string]interface{}{
				"type":    "function_call_output",
				"call_id": tc.ID,
				"output":  result,
			})
		}

		if r.chains(store) && turn.id != "" {
			// The server holds the context: send only the tool outputs.
			previousID = turn.id
			input = outputs
		} else {
			previousID = ""
			if turn.content != "" {
				input = append(input, map[string]interface{}{"role": "assistant", "content": turn.content})
			}
			input = append(input, turn.replay...)
			input = append(input, outputs...)
		}
	}
}

func executeTool(byName map[string]toolkit.Tool, tc tools.ToolCall) (string, string) {
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
	switch v := res.(type) {
	case string:
		return v, ""
	default:
		raw, _ := json.Marshal(v)
		return string(raw), ""
	}
}

// ---------------------------------------------------------------------------
// models.AgnoModelInterface

// Invoke sends a request (running the tool loop) and returns the final turn.
func (r *Responses) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	return r.run(ctx, messages, options)
}

// AInvoke runs Invoke in a goroutine.
func (r *Responses) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch := make(chan *models.MessageResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		defer close(errCh)
		resp, err := r.Invoke(ctx, messages, options...)
		if err != nil {
			errCh <- err
			return
		}
		ch <- resp
	}()
	return ch, errCh
}

// InvokeStream streams text deltas to the models.WithStreamingFunc callback.
func (r *Responses) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range options {
		o(call)
	}
	if call.StreamingFunc == nil {
		return errors.New("responses: streaming function required for InvokeStream")
	}
	_, err := r.run(ctx, messages, options)
	return err
}

// AInvokeStream streams text deltas as MessageResponse chunks on a channel.
func (r *Responses) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
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
		if _, err := r.run(ctx, messages, opts); err != nil {
			errCh <- err
		}
	}()
	return ch, errCh
}

// Package dashscope provides integration with Alibaba Cloud DashScope's
// OpenAI-compatible endpoint for Qwen models.
//
// DashScope offers a "compatible-mode" OpenAI API surface area, so we reuse the
// OpenAI-like implementation and add DashScope-specific defaults and request
// parameters (e.g., enable_thinking).
package dashscope

import (
	"context"
	"errors"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	likeopenai "github.com/devalexandre/agno-golang/agno/models/openai/like"
)

const (
	// DefaultBaseURL is the default base URL for DashScope OpenAI-compatible API.
	// See: https://dashscope.aliyun.com/
	DefaultBaseURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"

	// DefaultModelID matches the Python implementation default.
	DefaultModelID = "qwen-plus"
)

// WithEnableThinking enables DashScope "thinking" mode (native parameter).
func WithEnableThinking(enable bool) models.OptionClient {
	return func(o *models.ClientOptions) {
		if o.ClientParams == nil {
			o.ClientParams = map[string]interface{}{}
		}
		o.ClientParams["enable_thinking"] = enable
	}
}

// WithIncludeThoughts is an alias compatible with the Python implementation.
// If set, it takes precedence over WithEnableThinking.
func WithIncludeThoughts(include bool) models.OptionClient {
	return func(o *models.ClientOptions) {
		if o.ClientParams == nil {
			o.ClientParams = map[string]interface{}{}
		}
		o.ClientParams["include_thoughts"] = include
	}
}

// WithThinkingBudget sets DashScope "thinking_budget" (native parameter).
func WithThinkingBudget(budget int) models.OptionClient {
	return func(o *models.ClientOptions) {
		if o.ClientParams == nil {
			o.ClientParams = map[string]interface{}{}
		}
		o.ClientParams["thinking_budget"] = budget
	}
}

// DashScope is a small wrapper around an OpenAI-compatible client that injects
// DashScope native parameters via `request_params`.
type DashScope struct {
	inner           models.AgnoModelInterface
	enableThinking  bool
	includeThoughts *bool
	thinkingBudget  *int
}

// NewDashScopeChat creates a new DashScope client.
//
// If no API key is provided, it will look for DASHSCOPE_API_KEY or QWEN_API_KEY.
// If you're pointing BaseURL to a local OpenAI-compatible server (e.g., LM Studio),
// you can omit the API key.
func NewDashScopeChat(options ...models.OptionClient) (models.AgnoModelInterface, error) {
	// Collect options to check what's been set
	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}

	finalOptions := []models.OptionClient{
		models.WithBaseURL(DefaultBaseURL),
		models.WithID(DefaultModelID),
	}

	// Read DashScope-specific config from ClientParams
	var enableThinking bool
	var includeThoughts *bool
	var thinkingBudget *int
	if opts.ClientParams != nil {
		if v, ok := opts.ClientParams["enable_thinking"].(bool); ok {
			enableThinking = v
		}
		if v, ok := opts.ClientParams["include_thoughts"].(bool); ok {
			includeThoughts = &v
		}
		if v, ok := opts.ClientParams["thinking_budget"].(int); ok {
			thinkingBudget = &v
		} else if v, ok := opts.ClientParams["thinking_budget"].(int64); ok {
			val := int(v)
			thinkingBudget = &val
		} else if v, ok := opts.ClientParams["thinking_budget"].(float64); ok {
			val := int(v)
			thinkingBudget = &val
		}
	}

	// Get API key from environment if not provided
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("QWEN_API_KEY")
		}
	}

	// Add model ID if provided
	if opts.ID != "" {
		finalOptions = append(finalOptions, models.WithID(opts.ID))
	}
	// Allow BaseURL override (e.g., LM Studio)
	if opts.BaseURL != "" {
		finalOptions = append(finalOptions, models.WithBaseURL(opts.BaseURL))
	}

	// Only enforce API key if using the DashScope default endpoint
	baseURL := DefaultBaseURL
	if opts.BaseURL != "" {
		baseURL = opts.BaseURL
	}
	if baseURL == DefaultBaseURL && apiKey == "" {
		return nil, errors.New("DASHSCOPE_API_KEY (or QWEN_API_KEY) not set")
	}
	if apiKey != "" {
		finalOptions = append(finalOptions, models.WithAPIKey(apiKey))
	}

	inner, err := likeopenai.NewLikeOpenAIChat(finalOptions...)
	if err != nil {
		return nil, err
	}

	return &DashScope{
		inner:           inner,
		enableThinking:  enableThinking,
		includeThoughts: includeThoughts,
		thinkingBudget:  thinkingBudget,
	}, nil
}

func (d *DashScope) GetID() string { return d.inner.GetID() }

func (d *DashScope) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	options = d.withDashScopeParams(options)
	return d.inner.Invoke(ctx, messages, options...)
}

func (d *DashScope) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	options = d.withDashScopeParams(options)
	return d.inner.AInvoke(ctx, messages, options...)
}

func (d *DashScope) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	options = d.withDashScopeParams(options)
	return d.inner.InvokeStream(ctx, messages, options...)
}

func (d *DashScope) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	options = d.withDashScopeParams(options)
	return d.inner.AInvokeStream(ctx, messages, options...)
}

func (d *DashScope) withDashScopeParams(options []models.Option) []models.Option {
	enable := d.enableThinking
	if d.includeThoughts != nil {
		enable = *d.includeThoughts
	}
	// If a thinking budget is provided, enable thinking unless the caller explicitly disabled it
	// via include_thoughts=false.
	if d.thinkingBudget != nil && d.includeThoughts == nil && !enable {
		enable = true
	}

	req := map[string]interface{}{}
	if enable {
		req["enable_thinking"] = true
	}
	if d.thinkingBudget != nil {
		req["thinking_budget"] = *d.thinkingBudget
	}
	if len(req) == 0 {
		return options
	}
	return append(options, models.WithRequestParams(req))
}

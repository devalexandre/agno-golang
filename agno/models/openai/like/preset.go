package like

import (
	"context"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Preset describes an OpenAI-compatible provider: where it lives, which
// environment variable holds its key and which model is used by default.
// Provider packages (cerebras, deepinfra, moonshot, ...) are thin wrappers that
// build a Preset and call NewPreset.
type Preset struct {
	// BaseURL is the OpenAI-compatible endpoint (the SDK appends /chat/completions).
	BaseURL string
	// EnvAPIKey is read when no API key is given explicitly. Optional.
	EnvAPIKey string
	// DefaultModel is used when no model ID is given.
	DefaultModel string
	// FallbackAPIKey is sent when neither an explicit key nor EnvAPIKey is set
	// (local servers such as LM Studio or llama.cpp accept any value).
	FallbackAPIKey string
	// Headers are provider default headers; caller-supplied DefaultHeaders win
	// on conflicting keys.
	Headers http.Header
	// CallDefaults are prepended to every Invoke/InvokeStream call, so explicit
	// per-call options still override them.
	CallDefaults []models.Option
}

// ResolveClientOptions materializes the caller options into a ClientOptions
// value without applying package defaults. Provider wrappers use it to read
// ClientParams (account IDs, flags, ...) before building their Preset.
func ResolveClientOptions(options ...models.OptionClient) *models.ClientOptions {
	opts := &models.ClientOptions{}
	for _, option := range options {
		option(opts)
	}
	return opts
}

// NewPreset builds an OpenAI-compatible chat model from a provider preset.
// Caller options are applied after the preset defaults, so models.WithBaseURL,
// models.WithID and models.WithAPIKey always override them.
func NewPreset(p Preset, options ...models.OptionClient) (models.AgnoModelInterface, error) {
	opts := ResolveClientOptions(options...)

	model := opts.ID
	if model == "" {
		model = p.DefaultModel
	}
	apiKey := opts.APIKey
	if apiKey == "" && p.EnvAPIKey != "" {
		apiKey = os.Getenv(p.EnvAPIKey)
	}
	if apiKey == "" {
		apiKey = p.FallbackAPIKey
	}

	final := []models.OptionClient{
		models.WithBaseURL(p.BaseURL),
		models.WithID(model),
	}
	if apiKey != "" {
		final = append(final, models.WithAPIKey(apiKey))
	}
	final = append(final, options...)

	if len(p.Headers) > 0 {
		merged := http.Header{}
		for k, v := range p.Headers {
			merged[http.CanonicalHeaderKey(k)] = append([]string(nil), v...)
		}
		for k, v := range opts.DefaultHeaders {
			merged[http.CanonicalHeaderKey(k)] = append([]string(nil), v...)
		}
		final = append(final, models.WithDefaultHeaders(merged))
	}

	base, err := NewLikeOpenAIChat(final...)
	if err != nil {
		return nil, err
	}

	defaults := append([]models.Option(nil), p.CallDefaults...)
	if opts.MaxTokens != nil {
		defaults = append(defaults, models.WithMaxTokens(*opts.MaxTokens))
	}
	if len(defaults) == 0 {
		return base, nil
	}
	return &presetModel{AgnoModelInterface: base, defaults: defaults}, nil
}

// presetModel prepends provider call defaults to every request.
type presetModel struct {
	models.AgnoModelInterface
	defaults []models.Option
}

func (m *presetModel) with(options []models.Option) []models.Option {
	out := make([]models.Option, 0, len(m.defaults)+len(options))
	out = append(out, m.defaults...)
	return append(out, options...)
}

func (m *presetModel) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	return m.AgnoModelInterface.Invoke(ctx, messages, m.with(options)...)
}

func (m *presetModel) AInvoke(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return m.AgnoModelInterface.AInvoke(ctx, messages, m.with(options)...)
}

func (m *presetModel) InvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) error {
	return m.AgnoModelInterface.InvokeStream(ctx, messages, m.with(options)...)
}

func (m *presetModel) AInvokeStream(ctx context.Context, messages []models.Message, options ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return m.AgnoModelInterface.AInvokeStream(ctx, messages, m.with(options)...)
}

// GetClientOptions exposes the wrapped model's client options when available.
func (m *presetModel) GetClientOptions() *models.ClientOptions {
	if co, ok := m.AgnoModelInterface.(interface{ GetClientOptions() *models.ClientOptions }); ok {
		return co.GetClientOptions()
	}
	return nil
}

// Unwrap returns the underlying OpenAI-compatible model.
func (m *presetModel) Unwrap() models.AgnoModelInterface { return m.AgnoModelInterface }

// ThinkingParams returns the request params that toggle provider "thinking"
// mode through the nested {"thinking": {"type": "enabled"|"disabled"}} object
// used by Moonshot (Kimi K2.x) and Xiaomi MiMo.
func ThinkingParams(enabled bool) map[string]interface{} {
	mode := "disabled"
	if enabled {
		mode = "enabled"
	}
	return map[string]interface{}{"thinking": map[string]interface{}{"type": mode}}
}

package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Moderation categories reported by the OpenAI moderation endpoint.
const (
	CategorySexual                = "sexual"
	CategorySexualMinors          = "sexual/minors"
	CategoryHarassment            = "harassment"
	CategoryHarassmentThreatening = "harassment/threatening"
	CategoryHate                  = "hate"
	CategoryHateThreatening       = "hate/threatening"
	CategoryIllicit               = "illicit"
	CategoryIllicitViolent        = "illicit/violent"
	CategorySelfHarm              = "self-harm"
	CategorySelfHarmIntent        = "self-harm/intent"
	CategorySelfHarmInstructions  = "self-harm/instructions"
	CategoryViolence              = "violence"
	CategoryViolenceGraphic       = "violence/graphic"
)

// ModerationResult is the normalized outcome of a moderation call.
type ModerationResult struct {
	// Flagged reports whether the provider flagged the content.
	Flagged bool
	// Categories maps a category name to whether it was flagged.
	Categories map[string]bool
	// CategoryScores maps a category name to its confidence score.
	CategoryScores map[string]float64
}

// FlaggedCategories returns the flagged category names, sorted.
func (r *ModerationResult) FlaggedCategories() []string {
	if r == nil {
		return nil
	}
	flagged := make([]string, 0, len(r.Categories))
	for name, isFlagged := range r.Categories {
		if isFlagged {
			flagged = append(flagged, name)
		}
	}
	sort.Strings(flagged)
	return flagged
}

// Moderator moderates content through a provider. Implement it to plug a
// moderation provider other than OpenAI.
type Moderator interface {
	// Moderate classifies the content and returns the normalized result.
	Moderate(ctx context.Context, content string) (*ModerationResult, error)
	// ModelID returns the identifier of the moderation model in use.
	ModelID() string
}

// ===== OPENAI MODERATOR =====

// DefaultOpenAIModerationModel is the moderation model used by default.
const DefaultOpenAIModerationModel = "omni-moderation-latest"

// DefaultOpenAIModerationURL is the OpenAI moderations endpoint.
const DefaultOpenAIModerationURL = "https://api.openai.com/v1/moderations"

// OpenAIModerator moderates content through the OpenAI moderations endpoint.
type OpenAIModerator struct {
	// APIKey is the OpenAI API key. Defaults to the OPENAI_API_KEY env var.
	APIKey string
	// Model is the moderation model. Defaults to DefaultOpenAIModerationModel.
	Model string
	// BaseURL is the moderations endpoint. Defaults to DefaultOpenAIModerationURL.
	BaseURL string
	// HTTPClient is the client used for requests. Defaults to a 30s client.
	HTTPClient *http.Client
}

// OpenAIModeratorOption configures an OpenAIModerator.
type OpenAIModeratorOption func(*OpenAIModerator)

// WithModerationAPIKey sets the API key.
func WithModerationAPIKey(apiKey string) OpenAIModeratorOption {
	return func(m *OpenAIModerator) { m.APIKey = apiKey }
}

// WithModerationModel sets the moderation model.
func WithModerationModel(model string) OpenAIModeratorOption {
	return func(m *OpenAIModerator) { m.Model = model }
}

// WithModerationBaseURL sets the moderations endpoint, useful for compatible providers.
func WithModerationBaseURL(baseURL string) OpenAIModeratorOption {
	return func(m *OpenAIModerator) { m.BaseURL = baseURL }
}

// WithModerationHTTPClient sets the HTTP client used for requests.
func WithModerationHTTPClient(client *http.Client) OpenAIModeratorOption {
	return func(m *OpenAIModerator) { m.HTTPClient = client }
}

// NewOpenAIModerator creates a moderator backed by the OpenAI moderations endpoint.
func NewOpenAIModerator(opts ...OpenAIModeratorOption) *OpenAIModerator {
	m := &OpenAIModerator{
		APIKey:     os.Getenv("OPENAI_API_KEY"),
		Model:      DefaultOpenAIModerationModel,
		BaseURL:    DefaultOpenAIModerationURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.Model == "" {
		m.Model = DefaultOpenAIModerationModel
	}
	if m.BaseURL == "" {
		m.BaseURL = DefaultOpenAIModerationURL
	}
	if m.HTTPClient == nil {
		m.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return m
}

// ModelID returns the moderation model in use.
func (m *OpenAIModerator) ModelID() string { return m.Model }

// openAIModerationResponse is the payload returned by the moderations endpoint.
type openAIModerationResponse struct {
	Results []struct {
		Flagged        bool               `json:"flagged"`
		Categories     map[string]bool    `json:"categories"`
		CategoryScores map[string]float64 `json:"category_scores"`
	} `json:"results"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Moderate classifies the content through the OpenAI moderations endpoint.
func (m *OpenAIModerator) Moderate(ctx context.Context, content string) (*ModerationResult, error) {
	if m.APIKey == "" {
		return nil, fmt.Errorf("openai moderation: missing API key")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"model": m.Model,
		"input": content,
	})
	if err != nil {
		return nil, fmt.Errorf("openai moderation: encoding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("openai moderation: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.APIKey)

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai moderation: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openai moderation: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai moderation: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded openAIModerationResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("openai moderation: decoding response: %w", err)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return nil, fmt.Errorf("openai moderation: %s", decoded.Error.Message)
	}
	if len(decoded.Results) == 0 {
		return nil, fmt.Errorf("openai moderation: empty results")
	}

	result := decoded.Results[0]
	return &ModerationResult{
		Flagged:        result.Flagged,
		Categories:     result.Categories,
		CategoryScores: result.CategoryScores,
	}, nil
}

// ===== MODERATION GUARDRAIL =====

// ModerationConfig configures the moderation guardrail.
type ModerationConfig struct {
	// RaiseForCategories restricts the failure to the given categories.
	// When empty, any flagged category fails the check.
	RaiseForCategories []string
	// Threshold fails the check when any category score is above it, even if the
	// provider did not flag the content. Disabled when zero.
	Threshold float64
	// FailOpen lets the content through when the moderation call itself fails.
	FailOpen bool
}

// ModerationOption configures a ModerationGuardrail.
type ModerationOption func(*ModerationConfig)

// WithRaiseForCategories restricts the failure to the given categories.
func WithRaiseForCategories(categories ...string) ModerationOption {
	return func(c *ModerationConfig) { c.RaiseForCategories = categories }
}

// WithModerationThreshold fails the check when a category score exceeds the threshold.
func WithModerationThreshold(threshold float64) ModerationOption {
	return func(c *ModerationConfig) { c.Threshold = threshold }
}

// WithModerationFailOpen lets content through when the moderation provider is unavailable.
func WithModerationFailOpen() ModerationOption {
	return func(c *ModerationConfig) { c.FailOpen = true }
}

// ModerationGuardrail rejects content flagged by a moderation provider.
type ModerationGuardrail struct {
	moderator Moderator
	config    ModerationConfig
}

// NewModerationGuardrail creates a moderation guardrail on top of any Moderator.
func NewModerationGuardrail(moderator Moderator, opts ...ModerationOption) *ModerationGuardrail {
	cfg := ModerationConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &ModerationGuardrail{moderator: moderator, config: cfg}
}

// NewOpenAIModerationGuardrail creates a moderation guardrail backed by the
// OpenAI moderations endpoint. Use NewModerationGuardrail directly to combine a
// moderator with guardrail options such as WithRaiseForCategories.
func NewOpenAIModerationGuardrail(opts ...OpenAIModeratorOption) *ModerationGuardrail {
	return NewModerationGuardrail(NewOpenAIModerator(opts...))
}

// GetName returns the guardrail name.
func (m *ModerationGuardrail) GetName() string { return "ModerationGuardrail" }

// GetDescription returns a human-readable description.
func (m *ModerationGuardrail) GetDescription() string {
	model := ""
	if m.moderator != nil {
		model = m.moderator.ModelID()
	}
	return fmt.Sprintf("Moderates content using %s", model)
}

// Check moderates the data and rejects it when the provider flags it.
func (m *ModerationGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok || strings.TrimSpace(text) == "" {
		return nil
	}
	if m.moderator == nil {
		return nil
	}

	result, err := m.moderator.Moderate(ctx, text)
	if err != nil {
		if m.config.FailOpen {
			return nil
		}
		return err
	}

	violated := m.violatedCategories(result)
	if len(violated) == 0 {
		return nil
	}

	stage := StageInput
	if _, isOutput := data.(*Output); isOutput {
		stage = StageOutput
	}
	return &CheckError{
		Stage:     stage,
		Trigger:   TriggerModerationViolation,
		Message:   "Moderation violation detected",
		Guardrail: m.GetName(),
		AdditionalData: map[string]interface{}{
			"categories":       result.Categories,
			"category_scores":  result.CategoryScores,
			"flagged_category": violated,
		},
	}
}

// CheckInput moderates the input of a run.
func (m *ModerationGuardrail) CheckInput(ctx context.Context, input *Input) error {
	return m.Check(ctx, input)
}

// CheckOutput moderates the output of a run.
func (m *ModerationGuardrail) CheckOutput(ctx context.Context, output *Output) error {
	return m.Check(ctx, output)
}

// violatedCategories returns the categories that should fail the check.
func (m *ModerationGuardrail) violatedCategories(result *ModerationResult) []string {
	violated := map[string]bool{}

	if result.Flagged {
		if len(m.config.RaiseForCategories) == 0 {
			for _, category := range result.FlaggedCategories() {
				violated[category] = true
			}
			// The provider flagged the content without naming a category.
			if len(violated) == 0 {
				violated["flagged"] = true
			}
		} else {
			for _, category := range m.config.RaiseForCategories {
				if result.Categories[category] {
					violated[category] = true
				}
			}
		}
	}

	if m.config.Threshold > 0 {
		for category, score := range result.CategoryScores {
			if score < m.config.Threshold {
				continue
			}
			if len(m.config.RaiseForCategories) > 0 && !containsString(m.config.RaiseForCategories, category) {
				continue
			}
			violated[category] = true
		}
	}

	names := make([]string, 0, len(violated))
	for category := range violated {
		names = append(names, category)
	}
	sort.Strings(names)
	return names
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

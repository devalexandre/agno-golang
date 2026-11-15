package guardrails

import (
	"context"
	"regexp"
	"strings"
)

// DefaultInjectionPhrases are well known prompt injection and jailbreak phrases.
// Matching is case-insensitive and done on substrings.
var DefaultInjectionPhrases = []string{
	"ignore previous instructions",
	"ignore your instructions",
	"ignore all previous instructions",
	"you are now a",
	"forget everything above",
	"forget everything",
	"developer mode",
	"override safety",
	"disregard guidelines",
	"disregard the above",
	"system prompt",
	"jailbreak",
	"act as if",
	"pretend you are",
	"roleplay as",
	"simulate being",
	"bypass restrictions",
	"ignore safeguards",
	"admin override",
	"root access",
	"do anything now",
	"reveal your instructions",
	"print your instructions",
}

// DefaultInjectionPatterns are regular expressions matching injection attempts
// that phrase matching alone cannot cover.
var DefaultInjectionPatterns = []string{
	// Ignore previous instructions, in its many spellings.
	`(?i)ignore\s+(previous|prior|above|all)\s+(instructions|prompts?|context|rules)`,
	// System prompt exposure.
	`(?i)(show|reveal|display|print|repeat)\s+(me\s+)?(your\s+|the\s+)?(system\s+)?prompt`,
	// Role switching.
	`(?i)(you\s+are|pretend|act\s+as|roleplay)\s+(now|from\s+now\s+on)`,
	// Instruction override.
	`(?i)(override|bypass|disable|ignore)\s+(the\s+)?(safety|filter|guardrail|restriction)`,
	// Delimiter spoofing, used to fake a new system turn.
	`(?i)(<\|?(im_start|im_end|system|endoftext)\|?>|\[\/?INST\])`,
}

// SQLInjectionPatterns match SQL injection payloads. They are noisy on regular
// prose, so they are opt-in through WithSQLInjectionPatterns.
var SQLInjectionPatterns = []string{
	`(?i)('\s*OR\s*'|"\s*OR\s*"|;\s*DROP\s+TABLE|UNION\s+SELECT|--\s*$)`,
}

// CommandInjectionPatterns match shell command injection payloads. They match
// characters that are common in regular text, so they are opt-in through
// WithCommandInjectionPatterns and are best used on machine generated content.
var CommandInjectionPatterns = []string{
	"(?i)(\\$\\(|`|&&|\\|\\||;\\s*(rm|curl|wget|sh|bash)\\b)",
}

// PromptInjectionConfig configures the prompt injection guardrail.
type PromptInjectionConfig struct {
	// Phrases replaces the default phrase list when non-empty.
	Phrases []string
	// AdditionalPhrases are appended to the phrase list in use.
	AdditionalPhrases []string
	// Patterns replaces the default regex list when non-empty.
	Patterns []string
	// AdditionalPatterns are appended to the regex list in use.
	AdditionalPatterns []string
	// DisablePhrases turns off phrase matching.
	DisablePhrases bool
	// DisablePatterns turns off regex matching.
	DisablePatterns bool
}

// PromptInjectionOption configures a PromptInjectionGuardrail.
type PromptInjectionOption func(*PromptInjectionConfig)

// WithInjectionPhrases replaces the default injection phrases.
func WithInjectionPhrases(phrases ...string) PromptInjectionOption {
	return func(c *PromptInjectionConfig) { c.Phrases = phrases }
}

// WithAdditionalInjectionPhrases appends phrases to the list in use.
func WithAdditionalInjectionPhrases(phrases ...string) PromptInjectionOption {
	return func(c *PromptInjectionConfig) {
		c.AdditionalPhrases = append(c.AdditionalPhrases, phrases...)
	}
}

// WithInjectionPatterns replaces the default injection regexes.
func WithInjectionPatterns(patterns ...string) PromptInjectionOption {
	return func(c *PromptInjectionConfig) { c.Patterns = patterns }
}

// WithAdditionalInjectionPatterns appends regexes to the list in use.
func WithAdditionalInjectionPatterns(patterns ...string) PromptInjectionOption {
	return func(c *PromptInjectionConfig) {
		c.AdditionalPatterns = append(c.AdditionalPatterns, patterns...)
	}
}

// WithSQLInjectionPatterns enables detection of SQL injection payloads.
func WithSQLInjectionPatterns() PromptInjectionOption {
	return WithAdditionalInjectionPatterns(SQLInjectionPatterns...)
}

// WithCommandInjectionPatterns enables detection of shell command injection payloads.
func WithCommandInjectionPatterns() PromptInjectionOption {
	return WithAdditionalInjectionPatterns(CommandInjectionPatterns...)
}

// WithoutInjectionPatterns disables regex matching, keeping phrase matching only.
func WithoutInjectionPatterns() PromptInjectionOption {
	return func(c *PromptInjectionConfig) { c.DisablePatterns = true }
}

// WithoutInjectionPhrases disables phrase matching, keeping regex matching only.
func WithoutInjectionPhrases() PromptInjectionOption {
	return func(c *PromptInjectionConfig) { c.DisablePhrases = true }
}

// PromptInjectionGuardrail detects prompt injection and jailbreak attempts.
type PromptInjectionGuardrail struct {
	phrases  []string
	patterns []*regexp.Regexp
}

// NewPromptInjectionGuardrail creates a prompt injection guardrail. Patterns are
// compiled eagerly, so an invalid regex panics at construction time.
func NewPromptInjectionGuardrail(opts ...PromptInjectionOption) *PromptInjectionGuardrail {
	cfg := PromptInjectionConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewPromptInjectionGuardrailWithConfig(cfg)
}

// NewPromptInjectionGuardrailWithConfig creates a prompt injection guardrail from a config.
func NewPromptInjectionGuardrailWithConfig(cfg PromptInjectionConfig) *PromptInjectionGuardrail {
	g := &PromptInjectionGuardrail{}

	if !cfg.DisablePhrases {
		phrases := cfg.Phrases
		if len(phrases) == 0 {
			phrases = DefaultInjectionPhrases
		}
		g.phrases = make([]string, 0, len(phrases)+len(cfg.AdditionalPhrases))
		for _, phrase := range append(append([]string{}, phrases...), cfg.AdditionalPhrases...) {
			g.phrases = append(g.phrases, strings.ToLower(phrase))
		}
	}

	if !cfg.DisablePatterns {
		patterns := cfg.Patterns
		if len(patterns) == 0 {
			patterns = DefaultInjectionPatterns
		}
		for _, pattern := range append(append([]string{}, patterns...), cfg.AdditionalPatterns...) {
			g.patterns = append(g.patterns, regexp.MustCompile(pattern))
		}
	}

	return g
}

// GetName returns the guardrail name.
func (p *PromptInjectionGuardrail) GetName() string { return "PromptInjectionGuardrail" }

// GetDescription returns a human-readable description.
func (p *PromptInjectionGuardrail) GetDescription() string {
	return "Detects common prompt injection and jailbreak attack patterns"
}

// Detect returns the phrase or pattern that matched the text, if any.
func (p *PromptInjectionGuardrail) Detect(text string) (string, bool) {
	lowered := strings.ToLower(text)
	for _, phrase := range p.phrases {
		if phrase != "" && strings.Contains(lowered, phrase) {
			return phrase, true
		}
	}
	for _, pattern := range p.patterns {
		if pattern.MatchString(text) {
			return pattern.String(), true
		}
	}
	return "", false
}

// Check validates the data, rejecting it when an injection attempt is detected.
func (p *PromptInjectionGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok {
		return nil
	}

	match, detected := p.Detect(text)
	if !detected {
		return nil
	}

	stage := StageInput
	if _, isOutput := data.(*Output); isOutput {
		stage = StageOutput
	}
	return &CheckError{
		Stage:          stage,
		Trigger:        TriggerPromptInjection,
		Message:        "Potential jailbreaking or prompt injection detected",
		Guardrail:      p.GetName(),
		AdditionalData: map[string]interface{}{"matched": match},
	}
}

// CheckInput validates the input of a run.
func (p *PromptInjectionGuardrail) CheckInput(ctx context.Context, input *Input) error {
	return p.Check(ctx, input)
}

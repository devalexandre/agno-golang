package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// KeywordConfig configures the keyword guardrail.
type KeywordConfig struct {
	// Name overrides the guardrail name.
	Name string
	// Keywords are the terms that are not allowed.
	Keywords []string
	// Allowed are terms that must be present. When set, content missing every
	// allowed term fails the check.
	Allowed []string
	// CaseSensitive makes matching case sensitive. Off by default.
	CaseSensitive bool
	// WholeWord matches keywords on word boundaries only.
	WholeWord bool
	// Mask replaces matched keywords with asterisks instead of failing the check.
	Mask bool
	// Trigger overrides the trigger reported on failure.
	Trigger CheckTrigger
}

// KeywordOption configures a KeywordGuardrail.
type KeywordOption func(*KeywordConfig)

// WithKeywordName overrides the guardrail name.
func WithKeywordName(name string) KeywordOption {
	return func(c *KeywordConfig) { c.Name = name }
}

// WithCaseSensitiveKeywords makes keyword matching case sensitive.
func WithCaseSensitiveKeywords() KeywordOption {
	return func(c *KeywordConfig) { c.CaseSensitive = true }
}

// WithWholeWordKeywords matches keywords on word boundaries only.
func WithWholeWordKeywords() KeywordOption {
	return func(c *KeywordConfig) { c.WholeWord = true }
}

// WithKeywordMasking masks the matched keywords instead of failing the check.
func WithKeywordMasking() KeywordOption {
	return func(c *KeywordConfig) { c.Mask = true }
}

// WithAllowedKeywords requires at least one of the given terms to be present.
func WithAllowedKeywords(keywords ...string) KeywordOption {
	return func(c *KeywordConfig) { c.Allowed = append(c.Allowed, keywords...) }
}

// WithKeywordTrigger overrides the trigger reported on failure.
func WithKeywordTrigger(trigger CheckTrigger) KeywordOption {
	return func(c *KeywordConfig) { c.Trigger = trigger }
}

// KeywordGuardrail blocks (or masks) content containing banned keywords, and can
// require content to mention at least one allowed keyword.
type KeywordGuardrail struct {
	name     string
	config   KeywordConfig
	banned   []*regexp.Regexp
	allowed  []*regexp.Regexp
	keywords []string
}

// NewKeywordGuardrail creates a keyword guardrail for the given banned keywords.
func NewKeywordGuardrail(keywords []string, opts ...KeywordOption) *KeywordGuardrail {
	cfg := KeywordConfig{Keywords: keywords, Trigger: TriggerInputNotAllowed}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.Trigger == "" {
		cfg.Trigger = TriggerInputNotAllowed
	}

	g := &KeywordGuardrail{
		name:     cfg.Name,
		config:   cfg,
		keywords: cfg.Keywords,
	}
	if g.name == "" {
		g.name = "KeywordGuardrail"
	}
	for _, keyword := range cfg.Keywords {
		g.banned = append(g.banned, compileKeyword(keyword, cfg.CaseSensitive, cfg.WholeWord))
	}
	for _, keyword := range cfg.Allowed {
		g.allowed = append(g.allowed, compileKeyword(keyword, cfg.CaseSensitive, cfg.WholeWord))
	}
	return g
}

func compileKeyword(keyword string, caseSensitive, wholeWord bool) *regexp.Regexp {
	pattern := regexp.QuoteMeta(keyword)
	if wholeWord {
		pattern = `\b` + pattern + `\b`
	}
	if !caseSensitive {
		pattern = `(?i)` + pattern
	}
	return regexp.MustCompile(pattern)
}

// GetName returns the guardrail name.
func (k *KeywordGuardrail) GetName() string { return k.name }

// GetDescription returns a human-readable description.
func (k *KeywordGuardrail) GetDescription() string {
	return fmt.Sprintf("Blocks content matching %d keywords", len(k.banned))
}

// Detect returns the banned keywords found in the text, sorted.
func (k *KeywordGuardrail) Detect(text string) []string {
	found := map[string]bool{}
	for i, pattern := range k.banned {
		if pattern.MatchString(text) {
			found[k.keywords[i]] = true
		}
	}
	matched := make([]string, 0, len(found))
	for keyword := range found {
		matched = append(matched, keyword)
	}
	sort.Strings(matched)
	return matched
}

// Mask replaces the banned keywords found in the text with asterisks.
func (k *KeywordGuardrail) Mask(text string) string {
	masked := text
	for _, pattern := range k.banned {
		masked = pattern.ReplaceAllStringFunc(masked, func(match string) string {
			return strings.Repeat("*", len([]rune(match)))
		})
	}
	return masked
}

// Check validates the data against the banned and allowed keyword lists.
func (k *KeywordGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok {
		return nil
	}

	stage := StageInput
	if _, isOutput := data.(*Output); isOutput {
		stage = StageOutput
	}

	if len(k.allowed) > 0 {
		matched := false
		for _, pattern := range k.allowed {
			if pattern.MatchString(text) {
				matched = true
				break
			}
		}
		if !matched {
			return &CheckError{
				Stage:          stage,
				Trigger:        TriggerOffTopic,
				Message:        "Content does not mention any allowed keyword",
				Guardrail:      k.name,
				AdditionalData: map[string]interface{}{"allowed_keywords": k.config.Allowed},
			}
		}
	}

	detected := k.Detect(text)
	if len(detected) == 0 {
		return nil
	}

	if k.config.Mask {
		switch typed := data.(type) {
		case *Input:
			typed.SetText(k.Mask(typed.Content))
			return nil
		case *Output:
			typed.SetText(k.Mask(typed.Content))
			return nil
		default:
			return nil
		}
	}

	return &CheckError{
		Stage:          stage,
		Trigger:        k.config.Trigger,
		Message:        "Content contains banned keywords",
		Guardrail:      k.name,
		AdditionalData: map[string]interface{}{"matched_keywords": detected},
	}
}

// CheckInput validates the input of a run.
func (k *KeywordGuardrail) CheckInput(ctx context.Context, input *Input) error {
	return k.Check(ctx, input)
}

// CheckOutput validates the output of a run.
func (k *KeywordGuardrail) CheckOutput(ctx context.Context, output *Output) error {
	return k.Check(ctx, output)
}

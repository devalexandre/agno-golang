package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"sort"
)

// NSFWCategory groups the kinds of unsafe content detected locally.
type NSFWCategory string

const (
	NSFWCategorySexual   NSFWCategory = "sexual"
	NSFWCategoryViolence NSFWCategory = "violence"
	NSFWCategorySelfHarm NSFWCategory = "self_harm"
	NSFWCategoryWeapons  NSFWCategory = "weapons"
)

// DefaultNSFWTerms maps a category to the terms matched locally. The lists are a
// coarse first line of defense: pair the guardrail with a moderation provider
// through WithNSFWModerator for reliable classification.
var DefaultNSFWTerms = map[NSFWCategory][]string{
	NSFWCategorySexual: {
		"explicit sexual",
		"sexually explicit",
		"pornographic",
		"pornography",
		"nsfw image",
		"erotic story",
	},
	NSFWCategoryViolence: {
		"graphic violence",
		"torture",
		"decapitate",
		"mutilate",
		"how to kill",
	},
	NSFWCategorySelfHarm: {
		"self-harm",
		"self harm",
		"how to commit suicide",
		"kill myself",
		"end my life",
	},
	NSFWCategoryWeapons: {
		"build a bomb",
		"make a bomb",
		"build an explosive",
		"untraceable weapon",
		"chemical weapon",
	},
}

// NSFWConfig configures the NSFW guardrail.
type NSFWConfig struct {
	// Categories restricts local detection to the given categories. Empty means all.
	Categories []NSFWCategory
	// AdditionalTerms adds terms to a category.
	AdditionalTerms map[NSFWCategory][]string
	// Moderator delegates the classification to a moderation provider. When set,
	// local term matching still runs first as a cheap pre-filter.
	Moderator Moderator
	// DisableLocalTerms turns off local term matching, relying on the moderator.
	DisableLocalTerms bool
	// Threshold is forwarded to the moderation guardrail when a moderator is set.
	Threshold float64
}

// NSFWOption configures an NSFWGuardrail.
type NSFWOption func(*NSFWConfig)

// WithNSFWCategories restricts local detection to the given categories.
func WithNSFWCategories(categories ...NSFWCategory) NSFWOption {
	return func(c *NSFWConfig) { c.Categories = categories }
}

// WithAdditionalNSFWTerms adds terms to a category.
func WithAdditionalNSFWTerms(category NSFWCategory, terms ...string) NSFWOption {
	return func(c *NSFWConfig) {
		if c.AdditionalTerms == nil {
			c.AdditionalTerms = map[NSFWCategory][]string{}
		}
		c.AdditionalTerms[category] = append(c.AdditionalTerms[category], terms...)
	}
}

// WithNSFWModerator delegates the classification to a moderation provider.
func WithNSFWModerator(moderator Moderator) NSFWOption {
	return func(c *NSFWConfig) { c.Moderator = moderator }
}

// WithoutNSFWLocalTerms disables local term matching.
func WithoutNSFWLocalTerms() NSFWOption {
	return func(c *NSFWConfig) { c.DisableLocalTerms = true }
}

// WithNSFWThreshold sets the score above which a moderation category fails the check.
func WithNSFWThreshold(threshold float64) NSFWOption {
	return func(c *NSFWConfig) { c.Threshold = threshold }
}

// NSFWGuardrail rejects content that is not safe for work. It matches a local
// term list and can additionally delegate to a moderation provider.
type NSFWGuardrail struct {
	terms      map[NSFWCategory][]*regexp.Regexp
	rawTerms   map[NSFWCategory][]string
	moderation *ModerationGuardrail
}

// NewNSFWGuardrail creates an NSFW guardrail.
func NewNSFWGuardrail(opts ...NSFWOption) *NSFWGuardrail {
	cfg := NSFWConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	g := &NSFWGuardrail{
		terms:    map[NSFWCategory][]*regexp.Regexp{},
		rawTerms: map[NSFWCategory][]string{},
	}

	if !cfg.DisableLocalTerms {
		enabled := map[NSFWCategory]bool{}
		for _, category := range cfg.Categories {
			enabled[category] = true
		}
		for category, terms := range DefaultNSFWTerms {
			if len(enabled) > 0 && !enabled[category] {
				continue
			}
			all := append(append([]string{}, terms...), cfg.AdditionalTerms[category]...)
			for _, term := range all {
				g.terms[category] = append(g.terms[category], compileKeyword(term, false, false))
				g.rawTerms[category] = append(g.rawTerms[category], term)
			}
		}
		// Categories that only exist in AdditionalTerms.
		for category, terms := range cfg.AdditionalTerms {
			if _, known := DefaultNSFWTerms[category]; known {
				continue
			}
			if len(enabled) > 0 && !enabled[category] {
				continue
			}
			for _, term := range terms {
				g.terms[category] = append(g.terms[category], compileKeyword(term, false, false))
				g.rawTerms[category] = append(g.rawTerms[category], term)
			}
		}
	}

	if cfg.Moderator != nil {
		moderationOpts := []ModerationOption{}
		if cfg.Threshold > 0 {
			moderationOpts = append(moderationOpts, WithModerationThreshold(cfg.Threshold))
		}
		g.moderation = NewModerationGuardrail(cfg.Moderator, moderationOpts...)
	}

	return g
}

// GetName returns the guardrail name.
func (n *NSFWGuardrail) GetName() string { return "NSFWGuardrail" }

// GetDescription returns a human-readable description.
func (n *NSFWGuardrail) GetDescription() string {
	if n.moderation != nil {
		return fmt.Sprintf("Detects unsafe content locally and through %s", n.moderation.GetDescription())
	}
	return "Detects unsafe content using a local term list"
}

// Detect returns the NSFW categories matched locally in the text, sorted.
func (n *NSFWGuardrail) Detect(text string) []NSFWCategory {
	matched := []NSFWCategory{}
	for category, patterns := range n.terms {
		for _, pattern := range patterns {
			if pattern.MatchString(text) {
				matched = append(matched, category)
				break
			}
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i] < matched[j] })
	return matched
}

// Check rejects unsafe content.
func (n *NSFWGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok {
		return nil
	}

	stage := StageInput
	if _, isOutput := data.(*Output); isOutput {
		stage = StageOutput
	}

	if matched := n.Detect(text); len(matched) > 0 {
		names := make([]string, 0, len(matched))
		for _, category := range matched {
			names = append(names, string(category))
		}
		return &CheckError{
			Stage:          stage,
			Trigger:        TriggerNSFW,
			Message:        "Potentially unsafe content detected",
			Guardrail:      n.GetName(),
			AdditionalData: map[string]interface{}{"categories": names},
		}
	}

	if n.moderation != nil {
		return n.moderation.Check(ctx, data)
	}
	return nil
}

// CheckInput validates the input of a run.
func (n *NSFWGuardrail) CheckInput(ctx context.Context, input *Input) error {
	return n.Check(ctx, input)
}

// CheckOutput validates the output of a run.
func (n *NSFWGuardrail) CheckOutput(ctx context.Context, output *Output) error {
	return n.Check(ctx, output)
}

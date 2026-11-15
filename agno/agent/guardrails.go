package agent

import (
	"context"
	"time"

	"github.com/devalexandre/agno-golang/agno/guardrails"
)

// This file keeps the guardrail surface historically exposed by the agent
// package. The implementations live in agno/guardrails, which agents, teams and
// workflows share. Prefer importing that package directly in new code.

// Guardrail represents a reusable validation/policy rule.
type Guardrail = guardrails.Guardrail

// GuardrailFunc is a simple function-based guardrail implementation.
type GuardrailFunc = guardrails.Func

// GuardrailChain executes multiple guardrails in sequence.
type GuardrailChain = guardrails.Chain

// PromptInjectionGuardrail detects prompt injection and jailbreak attempts.
type PromptInjectionGuardrail = guardrails.PromptInjectionGuardrail

// InputLengthGuardrail validates input length.
type InputLengthGuardrail = guardrails.InputLengthGuardrail

// OutputContentGuardrail filters dangerous content from output.
type OutputContentGuardrail = guardrails.OutputContentGuardrail

// RateLimitGuardrail enforces rate limiting per user.
type RateLimitGuardrail = guardrails.RateLimitGuardrail

// LoopDetectionGuardrail detects runaway loops in agent execution.
type LoopDetectionGuardrail = guardrails.LoopDetectionGuardrail

// SemanticSimilarityGuardrail detects repetitive outputs.
type SemanticSimilarityGuardrail = guardrails.SemanticSimilarityGuardrail

// PIIDetectionGuardrail detects personally identifiable information.
type PIIDetectionGuardrail = guardrails.PIIDetectionGuardrail

// ModerationGuardrail rejects content flagged by a moderation provider.
type ModerationGuardrail = guardrails.ModerationGuardrail

// KeywordGuardrail blocks content containing banned keywords.
type KeywordGuardrail = guardrails.KeywordGuardrail

// NSFWGuardrail rejects content that is not safe for work.
type NSFWGuardrail = guardrails.NSFWGuardrail

// RunGuardrails executes a list of guardrails on data.
func RunGuardrails(ctx context.Context, gs []Guardrail, data interface{}) error {
	return guardrails.Run(ctx, gs, data)
}

// NewPromptInjectionGuardrail creates a guardrail detecting prompt injection attacks.
func NewPromptInjectionGuardrail(opts ...guardrails.PromptInjectionOption) *PromptInjectionGuardrail {
	return guardrails.NewPromptInjectionGuardrail(opts...)
}

// NewInputLengthGuardrail creates a guardrail limiting input length.
func NewInputLengthGuardrail(maxLength int) *InputLengthGuardrail {
	return guardrails.NewInputLengthGuardrail(maxLength)
}

// NewOutputContentGuardrail creates a guardrail filtering dangerous output.
func NewOutputContentGuardrail(extraPatterns ...string) *OutputContentGuardrail {
	return guardrails.NewOutputContentGuardrail(extraPatterns...)
}

// NewRateLimitGuardrail creates a rate limiting guardrail.
func NewRateLimitGuardrail(maxRequests int, windowSize time.Duration) *RateLimitGuardrail {
	return guardrails.NewRateLimitGuardrail(maxRequests, windowSize)
}

// NewLoopDetectionGuardrail creates a guardrail detecting runaway loops.
func NewLoopDetectionGuardrail(maxIterations int) *LoopDetectionGuardrail {
	return guardrails.NewLoopDetectionGuardrail(maxIterations)
}

// NewSemanticSimilarityGuardrail creates a guardrail detecting repetitive outputs.
func NewSemanticSimilarityGuardrail(maxSimilarity float64) *SemanticSimilarityGuardrail {
	return guardrails.NewSemanticSimilarityGuardrail(maxSimilarity)
}

// NewPIIDetectionGuardrail creates a guardrail detecting personally identifiable information.
func NewPIIDetectionGuardrail(opts ...guardrails.PIIOption) *PIIDetectionGuardrail {
	return guardrails.NewPIIDetectionGuardrail(opts...)
}

// NewOpenAIModerationGuardrail creates a guardrail moderating content through OpenAI.
func NewOpenAIModerationGuardrail(opts ...guardrails.OpenAIModeratorOption) *ModerationGuardrail {
	return guardrails.NewOpenAIModerationGuardrail(opts...)
}

// NewKeywordGuardrail creates a guardrail blocking the given keywords.
func NewKeywordGuardrail(keywords []string, opts ...guardrails.KeywordOption) *KeywordGuardrail {
	return guardrails.NewKeywordGuardrail(keywords, opts...)
}

// NewNSFWGuardrail creates a guardrail rejecting unsafe content.
func NewNSFWGuardrail(opts ...guardrails.NSFWOption) *NSFWGuardrail {
	return guardrails.NewNSFWGuardrail(opts...)
}

// NewDefaultInputGuardrails creates the default set of input guardrails.
func NewDefaultInputGuardrails() []Guardrail {
	return guardrails.NewDefaultInputGuardrails()
}

// NewDefaultOutputGuardrails creates the default set of output guardrails.
func NewDefaultOutputGuardrails() []Guardrail {
	return guardrails.NewDefaultOutputGuardrails()
}

// NewDefaultToolGuardrails creates the default set of tool guardrails.
func NewDefaultToolGuardrails() []Guardrail {
	return guardrails.NewDefaultToolGuardrails()
}

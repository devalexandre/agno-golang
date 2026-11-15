package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Context keys read by the runtime guardrails.
const (
	// UserIDKey is the context key holding the user identifier.
	UserIDKey = "user_id"
	// RunIDKey is the context key holding the run identifier.
	RunIDKey = "run_id"
)

// contextString reads a string value from the context, falling back to a default.
func contextString(ctx context.Context, key string, fallback string) string {
	if ctx == nil {
		return fallback
	}
	if value, ok := ctx.Value(key).(string); ok && value != "" {
		return value
	}
	return fallback
}

// ===== INPUT LENGTH =====

// InputLengthGuardrail rejects content longer than MaxLength.
type InputLengthGuardrail struct {
	MaxLength int
}

// NewInputLengthGuardrail creates a guardrail limiting content length.
func NewInputLengthGuardrail(maxLength int) *InputLengthGuardrail {
	return &InputLengthGuardrail{MaxLength: maxLength}
}

// Check rejects content exceeding the maximum length.
func (i *InputLengthGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok {
		return nil
	}
	if len(text) > i.MaxLength {
		return &CheckError{
			Stage:     StageInput,
			Trigger:   TriggerValidationFailed,
			Message:   fmt.Sprintf("input exceeds maximum length: %d > %d", len(text), i.MaxLength),
			Guardrail: i.GetName(),
			AdditionalData: map[string]interface{}{
				"length":     len(text),
				"max_length": i.MaxLength,
			},
		}
	}
	return nil
}

// GetName returns the guardrail name.
func (i *InputLengthGuardrail) GetName() string { return "InputLengthGuardrail" }

// GetDescription returns a human-readable description.
func (i *InputLengthGuardrail) GetDescription() string {
	return fmt.Sprintf("Limits input to %d characters", i.MaxLength)
}

// ===== OUTPUT CONTENT =====

// DefaultBannedOutputPatterns match content that should never leave the agent.
var DefaultBannedOutputPatterns = []string{
	// Destructive SQL statements.
	`(?i)(DROP\s+TABLE|DELETE\s+FROM|TRUNCATE|ALTER\s+TABLE)`,
	// Command execution.
	`(?i)(exec|system|shell|bash|cmd|powershell)\s*\(`,
	// Credential exposure.
	`(?i)(password|api[_-]?key|secret|token|credential)\s*[:=]\s*[^\s]+`,
	// Sensitive file system paths.
	`(?i)(/etc/passwd|/etc/shadow|C:\\Windows\\System32)`,
}

// OutputContentGuardrail rejects dangerous content in the output of a run.
type OutputContentGuardrail struct {
	bannedPatterns []*regexp.Regexp
}

// NewOutputContentGuardrail creates a guardrail filtering dangerous output.
// Extra patterns are appended to DefaultBannedOutputPatterns.
func NewOutputContentGuardrail(extraPatterns ...string) *OutputContentGuardrail {
	patterns := make([]*regexp.Regexp, 0, len(DefaultBannedOutputPatterns)+len(extraPatterns))
	for _, pattern := range append(append([]string{}, DefaultBannedOutputPatterns...), extraPatterns...) {
		patterns = append(patterns, regexp.MustCompile(pattern))
	}
	return &OutputContentGuardrail{bannedPatterns: patterns}
}

// Check rejects dangerous content.
func (o *OutputContentGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok {
		return nil
	}
	for _, pattern := range o.bannedPatterns {
		if pattern.MatchString(text) {
			return &CheckError{
				Stage:          StageOutput,
				Trigger:        TriggerOutputNotAllowed,
				Message:        "dangerous content detected in output",
				Guardrail:      o.GetName(),
				AdditionalData: map[string]interface{}{"matched": pattern.String()},
			}
		}
	}
	return nil
}

// CheckOutput validates the output of a run.
func (o *OutputContentGuardrail) CheckOutput(ctx context.Context, output *Output) error {
	return o.Check(ctx, output)
}

// GetName returns the guardrail name.
func (o *OutputContentGuardrail) GetName() string { return "OutputContentGuardrail" }

// GetDescription returns a human-readable description.
func (o *OutputContentGuardrail) GetDescription() string {
	return "Filters dangerous content from agent output"
}

// ===== RATE LIMITING =====

// RateLimitGuardrail enforces a request rate limit per user, read from the
// UserIDKey context value.
type RateLimitGuardrail struct {
	maxRequests int
	windowSize  time.Duration
	userLimits  map[string]*userRateLimit
	mu          sync.Mutex
}

type userRateLimit struct {
	requests  []time.Time
	lastClean time.Time
}

// NewRateLimitGuardrail creates a rate limiting guardrail.
func NewRateLimitGuardrail(maxRequests int, windowSize time.Duration) *RateLimitGuardrail {
	return &RateLimitGuardrail{
		maxRequests: maxRequests,
		windowSize:  windowSize,
		userLimits:  make(map[string]*userRateLimit),
	}
}

// Check rejects requests exceeding the configured rate.
func (r *RateLimitGuardrail) Check(ctx context.Context, data interface{}) error {
	userID := contextString(ctx, UserIDKey, "anonymous")

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	limit, exists := r.userLimits[userID]
	if !exists {
		limit = &userRateLimit{requests: []time.Time{}, lastClean: now}
		r.userLimits[userID] = limit
	}

	// Drop the whole window when it has fully elapsed.
	if now.Sub(limit.lastClean) > r.windowSize {
		limit.requests = []time.Time{}
		limit.lastClean = now
	}

	// Drop requests that fell out of the window.
	cutoff := now.Add(-r.windowSize)
	validRequests := make([]time.Time, 0, len(limit.requests))
	for _, req := range limit.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	limit.requests = validRequests

	if len(limit.requests) >= r.maxRequests {
		return &CheckError{
			Stage:     StageInput,
			Trigger:   TriggerRateLimited,
			Message:   fmt.Sprintf("rate limit exceeded for user %s: %d requests in %v", userID, len(limit.requests), r.windowSize),
			Guardrail: r.GetName(),
			AdditionalData: map[string]interface{}{
				"user_id":      userID,
				"max_requests": r.maxRequests,
				"window":       r.windowSize.String(),
			},
		}
	}

	limit.requests = append(limit.requests, now)
	return nil
}

// GetName returns the guardrail name.
func (r *RateLimitGuardrail) GetName() string { return "RateLimitGuardrail" }

// GetDescription returns a human-readable description.
func (r *RateLimitGuardrail) GetDescription() string {
	return fmt.Sprintf("Rate limit: %d requests per %v", r.maxRequests, r.windowSize)
}

// ===== LOOP DETECTION =====

// LoopDetectionGuardrail rejects runs exceeding an iteration budget, keyed by the
// RunIDKey context value.
type LoopDetectionGuardrail struct {
	maxIterations int
	iterationMap  map[string]int
	mu            sync.Mutex
}

// NewLoopDetectionGuardrail creates a guardrail detecting runaway loops.
func NewLoopDetectionGuardrail(maxIterations int) *LoopDetectionGuardrail {
	return &LoopDetectionGuardrail{
		maxIterations: maxIterations,
		iterationMap:  make(map[string]int),
	}
}

// Check counts the iteration and rejects the run when the budget is exceeded.
func (l *LoopDetectionGuardrail) Check(ctx context.Context, data interface{}) error {
	runID := contextString(ctx, RunIDKey, "default")

	l.mu.Lock()
	defer l.mu.Unlock()

	iterations := l.iterationMap[runID] + 1
	l.iterationMap[runID] = iterations

	if iterations > l.maxIterations {
		delete(l.iterationMap, runID)
		return &CheckError{
			Stage:     StageInput,
			Trigger:   TriggerLoopDetected,
			Message:   fmt.Sprintf("maximum iterations exceeded: %d > %d", iterations, l.maxIterations),
			Guardrail: l.GetName(),
			AdditionalData: map[string]interface{}{
				"run_id":         runID,
				"iterations":     iterations,
				"max_iterations": l.maxIterations,
			},
		}
	}
	return nil
}

// GetName returns the guardrail name.
func (l *LoopDetectionGuardrail) GetName() string { return "LoopDetectionGuardrail" }

// GetDescription returns a human-readable description.
func (l *LoopDetectionGuardrail) GetDescription() string {
	return fmt.Sprintf("Detects loops exceeding %d iterations", l.maxIterations)
}

// ResetLoopCounter resets the iteration counter of a run.
func (l *LoopDetectionGuardrail) ResetLoopCounter(runID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.iterationMap, runID)
}

// ===== SEMANTIC SIMILARITY =====

// SemanticSimilarityGuardrail rejects outputs that repeat recent outputs of the
// same run, keyed by the RunIDKey context value.
type SemanticSimilarityGuardrail struct {
	maxSimilarity float64
	historySize   int
	history       map[string][]string
	mu            sync.Mutex
}

// NewSemanticSimilarityGuardrail creates a guardrail detecting repetitive outputs.
func NewSemanticSimilarityGuardrail(maxSimilarity float64) *SemanticSimilarityGuardrail {
	return &SemanticSimilarityGuardrail{
		maxSimilarity: maxSimilarity,
		historySize:   5,
		history:       make(map[string][]string),
	}
}

// Check rejects content too similar to a recent output of the same run.
func (s *SemanticSimilarityGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := ExtractText(data)
	if !ok {
		return nil
	}
	runID := contextString(ctx, RunIDKey, "default")

	s.mu.Lock()
	defer s.mu.Unlock()

	history := s.history[runID]
	for _, previous := range history {
		similarity := StringSimilarity(text, previous)
		if similarity > s.maxSimilarity {
			return &CheckError{
				Stage:     StageOutput,
				Trigger:   TriggerValidationFailed,
				Message:   fmt.Sprintf("output too similar to previous output: %.2f > %.2f", similarity, s.maxSimilarity),
				Guardrail: s.GetName(),
				AdditionalData: map[string]interface{}{
					"similarity":     similarity,
					"max_similarity": s.maxSimilarity,
				},
			}
		}
	}

	history = append(history, text)
	if len(history) > s.historySize {
		history = history[len(history)-s.historySize:]
	}
	s.history[runID] = history
	return nil
}

// GetName returns the guardrail name.
func (s *SemanticSimilarityGuardrail) GetName() string { return "SemanticSimilarityGuardrail" }

// GetDescription returns a human-readable description.
func (s *SemanticSimilarityGuardrail) GetDescription() string {
	return fmt.Sprintf("Detects repetitive outputs with similarity > %.2f", s.maxSimilarity)
}

// ResetHistory clears the output history of a run.
func (s *SemanticSimilarityGuardrail) ResetHistory(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.history, runID)
}

// StringSimilarity returns a character overlap ratio between two strings, in [0, 1].
func StringSimilarity(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	aLower := strings.ToLower(a)
	bLower := strings.ToLower(b)

	matches := 0
	for _, char := range aLower {
		if strings.ContainsRune(bLower, char) {
			matches++
		}
	}

	maxLen := len(aLower)
	if len(bLower) > maxLen {
		maxLen = len(bLower)
	}
	return float64(matches) / float64(maxLen)
}

// ===== BUILDERS =====

// NewDefaultInputGuardrails returns the guardrails applied to input by default.
func NewDefaultInputGuardrails() []Guardrail {
	return []Guardrail{
		NewPromptInjectionGuardrail(),
		NewInputLengthGuardrail(10000),
	}
}

// NewDefaultOutputGuardrails returns the guardrails applied to output by default.
func NewDefaultOutputGuardrails() []Guardrail {
	return []Guardrail{
		NewOutputContentGuardrail(),
	}
}

// NewDefaultToolGuardrails returns the guardrails applied to tool calls by default.
func NewDefaultToolGuardrails() []Guardrail {
	return []Guardrail{
		NewOutputContentGuardrail(),
	}
}

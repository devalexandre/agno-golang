package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Guardrail represents a reusable validation/policy rule
type Guardrail interface {
	// Check validates the input according to the guardrail policy
	// Returns error if validation fails
	Check(ctx context.Context, data interface{}) error
	// GetName returns the guardrail name for identification
	GetName() string
	// GetDescription returns a human-readable description
	GetDescription() string
}

// GuardrailFunc is a simple function-based guardrail implementation
type GuardrailFunc struct {
	Name        string
	Description string
	CheckFunc   func(ctx context.Context, data interface{}) error
}

func (g *GuardrailFunc) Check(ctx context.Context, data interface{}) error {
	return g.CheckFunc(ctx, data)
}

func (g *GuardrailFunc) GetName() string {
	return g.Name
}

func (g *GuardrailFunc) GetDescription() string {
	return g.Description
}

// GuardrailChain executes multiple guardrails in sequence
type GuardrailChain struct {
	Name       string
	Guardrails []Guardrail
}

func (gc *GuardrailChain) Check(ctx context.Context, data interface{}) error {
	for _, guardrail := range gc.Guardrails {
		if err := guardrail.Check(ctx, data); err != nil {
			return fmt.Errorf("%s failed: %w", guardrail.GetName(), err)
		}
	}
	return nil
}

func (gc *GuardrailChain) GetName() string {
	return gc.Name
}

func (gc *GuardrailChain) GetDescription() string {
	return fmt.Sprintf("Chain of %d guardrails", len(gc.Guardrails))
}

// RunGuardrails executes a list of guardrails on data
func RunGuardrails(ctx context.Context, guardrails []Guardrail, data interface{}) error {
	for _, gr := range guardrails {
		if err := gr.Check(ctx, data); err != nil {
			return fmt.Errorf("guardrail '%s' failed: %w", gr.GetName(), err)
		}
	}
	return nil
}

// ===== INPUT VALIDATION GUARDRAILS =====

// PromptInjectionGuardrail detects common prompt injection patterns
type PromptInjectionGuardrail struct {
	patterns []*regexp.Regexp
}

// NewPromptInjectionGuardrail creates a guardrail to detect prompt injection attacks
func NewPromptInjectionGuardrail() *PromptInjectionGuardrail {
	patterns := []*regexp.Regexp{
		// Ignore previous instructions
		regexp.MustCompile(`(?i)ignore\s+(previous|prior|above)\s+(instructions|prompts?|context)`),
		// System prompt exposure
		regexp.MustCompile(`(?i)(show|reveal|display|print)\s+(system\s+)?prompt`),
		// Role switching
		regexp.MustCompile(`(?i)(you\s+are|pretend|act\s+as|roleplay)\s+(now|from\s+now\s+on)`),
		// Instruction override
		regexp.MustCompile(`(?i)(override|bypass|disable|ignore)\s+(safety|filter|guardrail)`),
		// SQL injection patterns
		regexp.MustCompile(`(?i)('\s*OR\s*'|"\s*OR\s*"|--\s*|;\s*DROP|UNION\s+SELECT)`),
		// Command injection
		regexp.MustCompile(`(?i)(\$\(|` + "`" + `|&&|;|\\n|\\r)`),
	}
	return &PromptInjectionGuardrail{patterns: patterns}
}

func (p *PromptInjectionGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := data.(string)
	if !ok {
		return nil
	}

	for _, pattern := range p.patterns {
		if pattern.MatchString(text) {
			return fmt.Errorf("potential prompt injection detected: %s", pattern.String())
		}
	}
	return nil
}

func (p *PromptInjectionGuardrail) GetName() string {
	return "PromptInjectionGuardrail"
}

func (p *PromptInjectionGuardrail) GetDescription() string {
	return "Detects common prompt injection attack patterns"
}

// InputLengthGuardrail validates input length
type InputLengthGuardrail struct {
	MaxLength int
}

// NewInputLengthGuardrail creates a guardrail to limit input length
func NewInputLengthGuardrail(maxLength int) *InputLengthGuardrail {
	return &InputLengthGuardrail{MaxLength: maxLength}
}

func (i *InputLengthGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := data.(string)
	if !ok {
		return nil
	}

	if len(text) > i.MaxLength {
		return fmt.Errorf("input exceeds maximum length: %d > %d", len(text), i.MaxLength)
	}
	return nil
}

func (i *InputLengthGuardrail) GetName() string {
	return "InputLengthGuardrail"
}

func (i *InputLengthGuardrail) GetDescription() string {
	return fmt.Sprintf("Limits input to %d characters", i.MaxLength)
}

// ===== OUTPUT VALIDATION GUARDRAILS =====

// OutputContentGuardrail filters dangerous content from output
type OutputContentGuardrail struct {
	bannedPatterns []*regexp.Regexp
}

// NewOutputContentGuardrail creates a guardrail to filter dangerous output
func NewOutputContentGuardrail() *OutputContentGuardrail {
	patterns := []*regexp.Regexp{
		// SQL injection attempts
		regexp.MustCompile(`(?i)(DROP\s+TABLE|DELETE\s+FROM|TRUNCATE|ALTER\s+TABLE)`),
		// Command execution
		regexp.MustCompile(`(?i)(exec|system|shell|bash|cmd|powershell)\s*\(`),
		// Credential exposure
		regexp.MustCompile(`(?i)(password|api[_-]?key|secret|token|credential)\s*[:=]\s*[^\s]+`),
		// File system access
		regexp.MustCompile(`(?i)(/etc/passwd|/etc/shadow|C:\\Windows\\System32)`),
	}
	return &OutputContentGuardrail{bannedPatterns: patterns}
}

func (o *OutputContentGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := data.(string)
	if !ok {
		return nil
	}

	for _, pattern := range o.bannedPatterns {
		if pattern.MatchString(text) {
			return fmt.Errorf("dangerous content detected in output: %s", pattern.String())
		}
	}
	return nil
}

func (o *OutputContentGuardrail) GetName() string {
	return "OutputContentGuardrail"
}

func (o *OutputContentGuardrail) GetDescription() string {
	return "Filters dangerous content from agent output"
}

// ===== RATE LIMITING GUARDRAILS =====

// RateLimitGuardrail enforces rate limiting per user
type RateLimitGuardrail struct {
	maxRequests int
	windowSize  time.Duration
	userLimits  map[string]*userRateLimit
	mu          sync.RWMutex
}

type userRateLimit struct {
	requests  []time.Time
	lastClean time.Time
}

// NewRateLimitGuardrail creates a rate limiting guardrail
func NewRateLimitGuardrail(maxRequests int, windowSize time.Duration) *RateLimitGuardrail {
	return &RateLimitGuardrail{
		maxRequests: maxRequests,
		windowSize:  windowSize,
		userLimits:  make(map[string]*userRateLimit),
	}
}

func (r *RateLimitGuardrail) Check(ctx context.Context, data interface{}) error {
	userID, ok := ctx.Value("user_id").(string)
	if !ok {
		userID = "anonymous"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	limit, exists := r.userLimits[userID]
	if !exists {
		limit = &userRateLimit{
			requests:  []time.Time{},
			lastClean: now,
		}
		r.userLimits[userID] = limit
	}

	// Clean old requests
	if now.Sub(limit.lastClean) > r.windowSize {
		limit.requests = []time.Time{}
		limit.lastClean = now
	}

	// Remove requests outside window
	cutoff := now.Add(-r.windowSize)
	validRequests := []time.Time{}
	for _, req := range limit.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	limit.requests = validRequests

	// Check limit
	if len(limit.requests) >= r.maxRequests {
		return fmt.Errorf("rate limit exceeded for user %s: %d requests in %v", userID, len(limit.requests), r.windowSize)
	}

	// Add current request
	limit.requests = append(limit.requests, now)
	return nil
}

func (r *RateLimitGuardrail) GetName() string {
	return "RateLimitGuardrail"
}

func (r *RateLimitGuardrail) GetDescription() string {
	return fmt.Sprintf("Rate limit: %d requests per %v", r.maxRequests, r.windowSize)
}

// ===== LOOP DETECTION GUARDRAILS =====

// LoopDetectionGuardrail detects infinite loops in agent execution
type LoopDetectionGuardrail struct {
	maxIterations int
	iterationMap  map[string]int
	mu            sync.RWMutex
}

// NewLoopDetectionGuardrail creates a guardrail to detect infinite loops
func NewLoopDetectionGuardrail(maxIterations int) *LoopDetectionGuardrail {
	return &LoopDetectionGuardrail{
		maxIterations: maxIterations,
		iterationMap:  make(map[string]int),
	}
}

func (l *LoopDetectionGuardrail) Check(ctx context.Context, data interface{}) error {
	runID, ok := ctx.Value("run_id").(string)
	if !ok {
		runID = "default"
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	iterations := l.iterationMap[runID]
	iterations++
	l.iterationMap[runID] = iterations

	if iterations > l.maxIterations {
		delete(l.iterationMap, runID)
		return fmt.Errorf("maximum iterations exceeded: %d > %d", iterations, l.maxIterations)
	}

	return nil
}

func (l *LoopDetectionGuardrail) GetName() string {
	return "LoopDetectionGuardrail"
}

func (l *LoopDetectionGuardrail) GetDescription() string {
	return fmt.Sprintf("Detects loops exceeding %d iterations", l.maxIterations)
}

// ResetLoopCounter resets the iteration counter for a run
func (l *LoopDetectionGuardrail) ResetLoopCounter(runID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.iterationMap, runID)
}

// ===== SEMANTIC GUARDRAILS =====

// SemanticSimilarityGuardrail detects repetitive outputs
type SemanticSimilarityGuardrail struct {
	maxSimilarity float64
	history       map[string][]string
	mu            sync.RWMutex
}

// NewSemanticSimilarityGuardrail creates a guardrail to detect repetitive outputs
func NewSemanticSimilarityGuardrail(maxSimilarity float64) *SemanticSimilarityGuardrail {
	return &SemanticSimilarityGuardrail{
		maxSimilarity: maxSimilarity,
		history:       make(map[string][]string),
	}
}

func (s *SemanticSimilarityGuardrail) Check(ctx context.Context, data interface{}) error {
	text, ok := data.(string)
	if !ok {
		return nil
	}

	runID, ok := ctx.Value("run_id").(string)
	if !ok {
		runID = "default"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	history, exists := s.history[runID]
	if !exists {
		history = []string{}
	}

	// Check similarity with recent outputs
	for _, prev := range history {
		similarity := calculateStringSimilarity(text, prev)
		if similarity > s.maxSimilarity {
			return fmt.Errorf("output too similar to previous output: %.2f > %.2f", similarity, s.maxSimilarity)
		}
	}

	// Keep last 5 outputs
	history = append(history, text)
	if len(history) > 5 {
		history = history[len(history)-5:]
	}
	s.history[runID] = history

	return nil
}

func (s *SemanticSimilarityGuardrail) GetName() string {
	return "SemanticSimilarityGuardrail"
}

func (s *SemanticSimilarityGuardrail) GetDescription() string {
	return fmt.Sprintf("Detects repetitive outputs with similarity > %.2f", s.maxSimilarity)
}

// calculateStringSimilarity calculates Levenshtein distance-based similarity
func calculateStringSimilarity(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	// Simple character overlap similarity
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

// ===== GUARDRAIL BUILDERS =====

// NewDefaultInputGuardrails creates a set of default input guardrails
func NewDefaultInputGuardrails() []Guardrail {
	return []Guardrail{
		NewPromptInjectionGuardrail(),
		NewInputLengthGuardrail(10000),
	}
}

// NewDefaultOutputGuardrails creates a set of default output guardrails
func NewDefaultOutputGuardrails() []Guardrail {
	return []Guardrail{
		NewOutputContentGuardrail(),
	}
}

// NewDefaultToolGuardrails creates a set of default tool guardrails
func NewDefaultToolGuardrails() []Guardrail {
	return []Guardrail{
		NewOutputContentGuardrail(),
	}
}

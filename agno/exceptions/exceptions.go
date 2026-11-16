// Package exceptions ports agno/exceptions.py: typed errors shared by the
// framework, provider error classification (rate limit, context window,
// non-retryable status codes) and secret redaction for error messages.
//
// Every error type carries an ErrorID (a stable machine-readable identifier)
// and, for AgnoError-derived errors, an HTTP-ish StatusCode. Use errors.As to
// inspect them and the Is* helpers for the common questions.
package exceptions

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Sentinel errors for conditions that carry no extra data. Typed errors below
// unwrap to these where it makes sense, so errors.Is works on both forms.
var (
	// ErrRunCancelled is returned when a run is cancelled.
	ErrRunCancelled = errors.New("operation cancelled by user")
	// ErrSessionBusy is the sentinel behind SessionBusyError.
	ErrSessionBusy = errors.New("session busy")
	// ErrRunNotFound is returned when a run_id cannot be found in the session (HTTP 404).
	ErrRunNotFound = errors.New("run not found")
	// ErrRunNotContinuable is returned when a run cannot be continued from its state (HTTP 409).
	ErrRunNotContinuable = errors.New("run not continuable")
	// ErrEval is returned when an evaluation fails.
	ErrEval = errors.New("evaluation failed")
)

// ---------------------------------------------------------------------------
// Secret redaction
// ---------------------------------------------------------------------------

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:sk|pk|rk|ghp|gho|xoxb|xoxp|pa)[-_][A-Za-z0-9\-_*•]{8,}`),
	regexp.MustCompile(`\bAIza[A-Za-z0-9\-_*•]{10,}`),
	regexp.MustCompile(`(?i)\b(?:hf|jina)_[A-Za-z0-9\-_*•]{8,}`),
	regexp.MustCompile(`\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA)[A-Z0-9]{12,}\b`),
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9\-._~+/]{8,}=*`),
	regexp.MustCompile(`(?i)\b(api[-_]?key|access[-_]?token|secret)\s*[=:]\s*\S+`),
	regexp.MustCompile(`(?i)\b(?:token|secret|key|password|signature|credential)\b[^A-Za-z0-9]{0,3}[A-Fa-f0-9]{32,}\b`),
}

var urlCredentialPattern = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://[^\s:/@]+):[^\s@]+@`)

// RedactSecrets replaces credential-like fragments in text with "[redacted]".
func RedactSecrets(text string) string {
	text = urlCredentialPattern.ReplaceAllString(text, "${1}:[redacted]@")
	for _, p := range secretPatterns {
		if p.NumSubexp() > 0 {
			text = p.ReplaceAllStringFunc(text, func(m string) string {
				sub := p.FindStringSubmatch(m)
				if len(sub) > 1 && sub[1] != "" {
					return sub[1] + "=[redacted]"
				}
				return "[redacted]"
			})
			continue
		}
		text = p.ReplaceAllString(text, "[redacted]")
	}
	return text
}

// ---------------------------------------------------------------------------
// Agent run control errors
// ---------------------------------------------------------------------------

// AgentRunError mirrors AgentRunException: a tool can return it to steer the run.
type AgentRunError struct {
	Err           error
	UserMessage   string
	AgentMessage  string
	Messages      []map[string]interface{}
	StopExecution bool
	ErrorID       string
}

func (e *AgentRunError) Error() string {
	if e.Err == nil {
		return e.ErrorID
	}
	return e.Err.Error()
}

func (e *AgentRunError) Unwrap() error { return e.Err }

// Type returns the error type string ("agent_run_error").
func (e *AgentRunError) Type() string { return "agent_run_error" }

// NewAgentRunError builds a generic AgentRunError.
func NewAgentRunError(err error, stop bool) *AgentRunError {
	return &AgentRunError{Err: err, StopExecution: stop, ErrorID: "agent_run_error"}
}

// NewRetryAgentRun signals that a tool call should be retried (stop_execution=false).
func NewRetryAgentRun(err error, userMessage, agentMessage string) *AgentRunError {
	return &AgentRunError{Err: err, UserMessage: userMessage, AgentMessage: agentMessage, ErrorID: "retry_agent_run_error"}
}

// NewStopAgentRun signals that the agent should stop executing entirely.
func NewStopAgentRun(err error, userMessage, agentMessage string) *AgentRunError {
	return &AgentRunError{Err: err, UserMessage: userMessage, AgentMessage: agentMessage, StopExecution: true, ErrorID: "stop_agent_run_error"}
}

// IsStopAgentRun reports whether err asks the agent to stop.
func IsStopAgentRun(err error) bool {
	var e *AgentRunError
	return errors.As(err, &e) && e.StopExecution
}

// IsRetryAgentRun reports whether err asks the agent to retry the tool call.
func IsRetryAgentRun(err error) bool {
	var e *AgentRunError
	return errors.As(err, &e) && e.ErrorID == "retry_agent_run_error"
}

// ---------------------------------------------------------------------------
// AgnoError hierarchy
// ---------------------------------------------------------------------------

// AgnoError is the base internal error with a status code.
type AgnoError struct {
	Message    string
	StatusCode int
	ErrType    string
	ErrorID    string
	Err        error // optional wrapped cause
}

func (e *AgnoError) Error() string { return e.Message }

func (e *AgnoError) Unwrap() error { return e.Err }

// Type returns the error type string.
func (e *AgnoError) Type() string { return e.ErrType }

// NewAgnoError builds an AgnoError; statusCode 0 defaults to 500.
func NewAgnoError(message string, statusCode int) *AgnoError {
	if statusCode == 0 {
		statusCode = 500
	}
	return &AgnoError{Message: message, StatusCode: statusCode, ErrType: "agno_error", ErrorID: "agno_error"}
}

// StatusCoder is implemented by every error carrying an HTTP status code.
type StatusCoder interface {
	error
	HTTPStatus() int
}

// HTTPStatus implements StatusCoder.
func (e *AgnoError) HTTPStatus() int { return e.StatusCode }

// ModelAuthenticationError is raised when model authentication fails.
type ModelAuthenticationError struct {
	AgnoError
	ModelName string
}

// NewModelAuthenticationError builds a ModelAuthenticationError (default 401).
func NewModelAuthenticationError(message string, modelName string) *ModelAuthenticationError {
	return &ModelAuthenticationError{
		AgnoError: AgnoError{Message: message, StatusCode: 401, ErrType: "model_authentication_error", ErrorID: "model_authentication_error"},
		ModelName: modelName,
	}
}

// ProviderErrorKind distinguishes the ModelProviderError subclasses.
type ProviderErrorKind string

const (
	ProviderErrorGeneric       ProviderErrorKind = "model_provider_error"
	ProviderErrorRateLimit     ProviderErrorKind = "model_rate_limit_error"
	ProviderErrorContextWindow ProviderErrorKind = "context_window_exceeded_error"
)

// NonRetryableStatusCodes are provider status codes that retrying cannot fix.
var NonRetryableStatusCodes = map[int]bool{400: true, 401: true, 403: true, 404: true, 413: true, 422: true}

// RateLimitStatusCodes are classified as rate-limit errors.
var RateLimitStatusCodes = map[int]bool{429: true, 529: true}

// ContextWindowPatterns are lowercase substrings that identify a context-window overflow.
var ContextWindowPatterns = []string{
	"context_length_exceeded",
	"context window",
	"maximum context length",
	"token limit",
	"max_tokens",
	"too many tokens",
	"payload too large",
	"content_too_large",
	"request too large",
	"input too long",
	"prompt is too long",
	"prompt too long",
	"exceeds the model",
	"token count exceeds",
	"exceeds the maximum number of tokens",
}

// ModelProviderError is returned when a model provider returns an error. Kind
// records the subclass (generic, rate limit, context window).
type ModelProviderError struct {
	AgnoError
	ModelName string
	ModelID   string
	Kind      ProviderErrorKind
}

// NewModelProviderError builds a generic provider error; statusCode 0 defaults to 502.
func NewModelProviderError(message string, statusCode int, modelName, modelID string) *ModelProviderError {
	if statusCode == 0 {
		statusCode = 502
	}
	return &ModelProviderError{
		AgnoError: AgnoError{Message: message, StatusCode: statusCode, ErrType: "model_provider_error", ErrorID: string(ProviderErrorGeneric)},
		ModelName: modelName,
		ModelID:   modelID,
		Kind:      ProviderErrorGeneric,
	}
}

// NewModelRateLimitError builds a rate-limit provider error (default 429).
func NewModelRateLimitError(message string, statusCode int, modelName, modelID string) *ModelProviderError {
	if statusCode == 0 {
		statusCode = 429
	}
	e := NewModelProviderError(message, statusCode, modelName, modelID)
	e.Kind = ProviderErrorRateLimit
	e.ErrorID = string(ProviderErrorRateLimit)
	return e
}

// NewContextWindowExceededError builds a context-window provider error (default 400).
func NewContextWindowExceededError(message string, statusCode int, modelName, modelID string) *ModelProviderError {
	if statusCode == 0 {
		statusCode = 400
	}
	e := NewModelProviderError(message, statusCode, modelName, modelID)
	e.Kind = ProviderErrorContextWindow
	e.ErrorID = string(ProviderErrorContextWindow)
	return e
}

// Classify re-classifies a generic provider error into a rate-limit or
// context-window error based on its status code and message. Already
// specific errors are returned unchanged.
func Classify(e *ModelProviderError) *ModelProviderError {
	if e == nil || e.Kind != ProviderErrorGeneric {
		return e
	}
	if RateLimitStatusCodes[e.StatusCode] {
		return NewModelRateLimitError(e.Message, e.StatusCode, e.ModelName, e.ModelID)
	}
	msg := strings.ToLower(e.Message)
	for _, p := range ContextWindowPatterns {
		if strings.Contains(msg, p) {
			return NewContextWindowExceededError(e.Message, e.StatusCode, e.ModelName, e.ModelID)
		}
	}
	return e
}

// IsRetryable reports whether re-sending the same request could succeed.
func (e *ModelProviderError) IsRetryable() bool {
	if e.Kind == ProviderErrorContextWindow {
		return false
	}
	if e.Kind == ProviderErrorRateLimit {
		return true
	}
	return !NonRetryableStatusCodes[e.StatusCode]
}

// IsRateLimit reports whether err is (or wraps) a rate-limit provider error.
func IsRateLimit(err error) bool {
	var e *ModelProviderError
	return errors.As(err, &e) && Classify(e).Kind == ProviderErrorRateLimit
}

// IsContextWindowExceeded reports whether err is (or wraps) a context-window error.
func IsContextWindowExceeded(err error) bool {
	var e *ModelProviderError
	return errors.As(err, &e) && Classify(e).Kind == ProviderErrorContextWindow
}

// IsRetryable reports whether err may succeed when retried. Errors that are
// not provider errors are considered retryable unless they are input/output
// check errors, authentication errors or cancellations.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var pe *ModelProviderError
	if errors.As(err, &pe) {
		return Classify(pe).IsRetryable()
	}
	var ae *ModelAuthenticationError
	if errors.As(err, &ae) {
		return false
	}
	var ic *InputCheckError
	if errors.As(err, &ic) {
		return false
	}
	var oc *OutputCheckError
	if errors.As(err, &oc) {
		return false
	}
	if errors.Is(err, ErrRunCancelled) {
		return false
	}
	var ee *EmbeddingError
	if errors.As(err, &ee) {
		return ee.IsRetryable()
	}
	return true
}

// ---------------------------------------------------------------------------
// Embedding errors
// ---------------------------------------------------------------------------

// EmbeddingError is raised when an embedder fails to produce an embedding.
type EmbeddingError struct {
	AgnoError
	ProviderStatusCode int // 0 when unknown
	ModelID            string
	Provider           string
}

// NewEmbeddingError builds an EmbeddingError; providerStatus 0 means unknown.
func NewEmbeddingError(message string, providerStatus int, modelID, provider string) *EmbeddingError {
	status := providerStatus
	if status == 0 {
		status = 502
	}
	return &EmbeddingError{
		AgnoError:          AgnoError{Message: message, StatusCode: status, ErrType: "embedding_error", ErrorID: "embedding_error"},
		ProviderStatusCode: providerStatus,
		ModelID:            modelID,
		Provider:           provider,
	}
}

var (
	embedAuthPatterns      = []string{"api key", "api_key", "unauthorized", "authentication", "invalid_api_key", "permission denied", "forbidden"}
	embedRateLimitPatterns = []string{"rate limit", "rate_limit", "too many requests", "quota", "429"}
	embedTooLargePatterns  = []string{"maximum context length", "too long", "too large", "exceeds", "max_tokens", "token limit"}
)

func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// Reason is one of "authentication", "rate_limit", "content_too_large", "unknown".
func (e *EmbeddingError) Reason() string {
	msg := strings.ToLower(e.Message)
	sc := e.ProviderStatusCode
	switch {
	case sc == 429:
		return "rate_limit"
	case sc == 401 || sc == 403:
		return "authentication"
	case sc >= 500 && sc < 600:
		return "unknown"
	case containsAny(msg, embedRateLimitPatterns):
		return "rate_limit"
	case containsAny(msg, embedAuthPatterns):
		return "authentication"
	case containsAny(msg, embedTooLargePatterns):
		return "content_too_large"
	}
	return "unknown"
}

// RecoveryHint is a short, actionable next step derived from Reason.
func (e *EmbeddingError) RecoveryHint() string {
	switch e.Reason() {
	case "authentication":
		return "Check the embedder's API key and permissions."
	case "rate_limit":
		return "The embedding provider rate-limited this request; wait for the limit to reset, or lower the embedder batch size."
	case "content_too_large":
		return "One or more chunks exceed the embedder's input limit; reduce the chunk size."
	}
	return "If the failure persists, check the embedder configuration."
}

// IsRetryable is false for authentication and oversized-input failures.
func (e *EmbeddingError) IsRetryable() bool {
	r := e.Reason()
	return r != "authentication" && r != "content_too_large"
}

// SafeMessage is the provider message with credential-like fragments removed.
func (e *EmbeddingError) SafeMessage() string { return RedactSecrets(e.Message) }

// ---------------------------------------------------------------------------
// Guardrail check errors
// ---------------------------------------------------------------------------

// CheckTrigger enumerates guardrail triggers.
type CheckTrigger string

const (
	CheckOffTopic         CheckTrigger = "off_topic"
	CheckInputNotAllowed  CheckTrigger = "input_not_allowed"
	CheckOutputNotAllowed CheckTrigger = "output_not_allowed"
	CheckValidationFailed CheckTrigger = "validation_failed"
	CheckPromptInjection  CheckTrigger = "prompt_injection"
	CheckPIIDetected      CheckTrigger = "pii_detected"
)

// InputCheckError is raised when an input check fails. Never retried.
type InputCheckError struct {
	Message        string
	Trigger        CheckTrigger
	AdditionalData map[string]interface{}
}

func (e *InputCheckError) Error() string { return e.Message }

// ErrorID returns the trigger value.
func (e *InputCheckError) ErrorID() string { return string(e.Trigger) }

// HTTPStatus implements StatusCoder (400).
func (e *InputCheckError) HTTPStatus() int { return 400 }

// NewInputCheckError builds an InputCheckError; empty trigger defaults to input_not_allowed.
func NewInputCheckError(message string, trigger CheckTrigger, data map[string]interface{}) *InputCheckError {
	if trigger == "" {
		trigger = CheckInputNotAllowed
	}
	return &InputCheckError{Message: message, Trigger: trigger, AdditionalData: data}
}

// NewUnsupportedMediaError reports media passed to an agent that cannot take it.
func NewUnsupportedMediaError(message string, media []string) *InputCheckError {
	if media == nil {
		media = []string{}
	}
	return NewInputCheckError(message, CheckInputNotAllowed, map[string]interface{}{"unsupported_media": media})
}

// OutputCheckError is raised when an output check fails.
type OutputCheckError struct {
	Message        string
	Trigger        CheckTrigger
	AdditionalData map[string]interface{}
}

func (e *OutputCheckError) Error() string { return e.Message }

// ErrorID returns the trigger value.
func (e *OutputCheckError) ErrorID() string { return string(e.Trigger) }

// NewOutputCheckError builds an OutputCheckError; empty trigger defaults to output_not_allowed.
func NewOutputCheckError(message string, trigger CheckTrigger, data map[string]interface{}) *OutputCheckError {
	if trigger == "" {
		trigger = CheckOutputNotAllowed
	}
	return &OutputCheckError{Message: message, Trigger: trigger, AdditionalData: data}
}

// RetryableModelProviderError carries guidance for retrying a model call.
type RetryableModelProviderError struct {
	OriginalError        string
	RetryGuidanceMessage string
}

func (e *RetryableModelProviderError) Error() string {
	if e.RetryGuidanceMessage != "" {
		return e.RetryGuidanceMessage
	}
	return e.OriginalError
}

// ---------------------------------------------------------------------------
// Infrastructure errors
// ---------------------------------------------------------------------------

// RemoteServerUnavailableError is raised when a remote server is unreachable (503).
type RemoteServerUnavailableError struct {
	AgnoError
	BaseURL string
}

// NewRemoteServerUnavailableError builds the error, wrapping the original cause.
func NewRemoteServerUnavailableError(message, baseURL string, cause error) *RemoteServerUnavailableError {
	return &RemoteServerUnavailableError{
		AgnoError: AgnoError{Message: message, StatusCode: 503, ErrType: "remote_server_unavailable_error", ErrorID: "remote_server_unavailable_error", Err: cause},
		BaseURL:   baseURL,
	}
}

// NewPathSecurityError reports a rejected user-supplied path (400).
func NewPathSecurityError(message string) *AgnoError {
	if message == "" {
		message = "Path security violation"
	}
	return &AgnoError{Message: message, StatusCode: 400, ErrType: "path_security_error", ErrorID: "path_security_error"}
}

// NewComponentRehydrationError reports a persisted component that cannot be rebuilt (422).
func NewComponentRehydrationError(message string) *AgnoError {
	return &AgnoError{Message: message, StatusCode: 422, ErrType: "component_rehydration_error", ErrorID: "component_rehydration_error"}
}

// SchemaMismatchError reports a table whose schema does not match expectations.
type SchemaMismatchError struct {
	AgnoError
	TableName         string
	MigrationRequired bool
}

// NewSchemaMismatchError builds a SchemaMismatchError (500).
func NewSchemaMismatchError(table, message string) *SchemaMismatchError {
	if message == "" {
		message = fmt.Sprintf("Table %s has an invalid schema: it does not match what this version of Agno expects.", table)
	}
	return &SchemaMismatchError{
		AgnoError: AgnoError{Message: message, StatusCode: 500, ErrType: "schema_mismatch_error", ErrorID: "schema_mismatch_error"},
		TableName: table,
	}
}

// NewMigrationRequiredError builds a SchemaMismatchError that a migration can fix.
func NewMigrationRequiredError(table, message string) *SchemaMismatchError {
	if message == "" {
		message = fmt.Sprintf("Table %s has an invalid schema: it does not match what this version of Agno expects. "+
			"If this database was created by an older version of Agno, apply the pending migrations.", table)
	}
	e := NewSchemaMismatchError(table, message)
	e.ErrType = "migration_required_error"
	e.ErrorID = "migration_required_error"
	e.MigrationRequired = true
	return e
}

// SessionBusyError is raised when a turn starts on a session with a turn in flight (409).
type SessionBusyError struct {
	AgnoError
	RunID     string
	SessionID string
}

// NewSessionBusyError builds a SessionBusyError.
func NewSessionBusyError(message, runID, sessionID string) *SessionBusyError {
	if message == "" {
		message = fmt.Sprintf("session %s already has a turn in flight", sessionID)
	}
	return &SessionBusyError{
		AgnoError: AgnoError{Message: message, StatusCode: 409, ErrType: "session_busy", ErrorID: "session_busy", Err: ErrSessionBusy},
		RunID:     runID,
		SessionID: sessionID,
	}
}

// IsSessionBusy reports whether err is a SessionBusyError.
func IsSessionBusy(err error) bool { return errors.Is(err, ErrSessionBusy) }

// HTTPStatusOf maps an error to an HTTP status code (500 when unknown).
func HTTPStatusOf(err error) int {
	if err == nil {
		return 200
	}
	var sc StatusCoder
	if errors.As(err, &sc) {
		return sc.HTTPStatus()
	}
	switch {
	case errors.Is(err, ErrRunNotFound):
		return 404
	case errors.Is(err, ErrRunNotContinuable), errors.Is(err, ErrSessionBusy):
		return 409
	}
	return 500
}

package models

import (
	"context"
	"strings"
	"sync"
)

// errorClass classifies a provider error for fallback routing.
type errorClass int

const (
	classGeneric errorClass = iota
	classRateLimit
	classContextOverflow
	classNonRetryable
)

// classifyError inspects a provider error message and classifies it. Go SDK
// errors do not share a common typed hierarchy, so this uses message heuristics.
func classifyError(err error) errorClass {
	if err == nil {
		return classGeneric
	}
	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "429"),
		strings.Contains(msg, "529"),
		strings.Contains(msg, "rate limit"),
		strings.Contains(msg, "rate_limit"),
		strings.Contains(msg, "too many requests"):
		return classRateLimit

	case strings.Contains(msg, "context window"),
		strings.Contains(msg, "context length"),
		strings.Contains(msg, "maximum context"),
		strings.Contains(msg, "context_length_exceeded"),
		strings.Contains(msg, "too long"):
		return classContextOverflow

	// Non-retryable client errors (auth/permission/bad request) are configuration
	// bugs the developer must see — they should not be masked by general fallbacks.
	case strings.Contains(msg, "401"),
		strings.Contains(msg, "403"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "permission denied"),
		strings.Contains(msg, "invalid api key"),
		strings.Contains(msg, "invalid_api_key"):
		return classNonRetryable
	}

	return classGeneric
}

// FallbackCallback is invoked when a fallback model is activated.
// Signature: (primaryModelID, fallbackModelID, error).
type FallbackCallback func(primaryID, fallbackID string, err error)

// FallbackConfig configures fallback model behavior.
//
// Example:
//
//	cfg := models.FallbackConfig{
//	    OnError:           []models.AgnoModelInterface{claude},
//	    OnRateLimit:       []models.AgnoModelInterface{gptMini},
//	    OnContextOverflow: []models.AgnoModelInterface{claude},
//	}
type FallbackConfig struct {
	// OnError lists models tried when the primary fails with a generic/retryable error.
	OnError []AgnoModelInterface
	// OnRateLimit lists models tried specifically on rate-limit (429/529) errors.
	OnRateLimit []AgnoModelInterface
	// OnContextOverflow lists models tried specifically on context-window errors.
	OnContextOverflow []AgnoModelInterface
	// Callback, if set, is invoked each time a fallback model is activated.
	Callback FallbackCallback
}

// HasFallbacks reports whether any fallback models are configured.
func (c *FallbackConfig) HasFallbacks() bool {
	return len(c.OnError) > 0 || len(c.OnRateLimit) > 0 || len(c.OnContextOverflow) > 0
}

// selectFallbacks returns the appropriate fallback list for the given error.
func (c *FallbackConfig) selectFallbacks(err error) []AgnoModelInterface {
	switch classifyError(err) {
	case classRateLimit:
		if len(c.OnRateLimit) > 0 {
			return c.OnRateLimit
		}
	case classContextOverflow:
		if len(c.OnContextOverflow) > 0 {
			return c.OnContextOverflow
		}
	case classNonRetryable:
		// Never mask configuration bugs with general fallbacks.
		return nil
	}
	return c.OnError
}

// ModelMetrics holds counters tracked by a FallbackModel.
type ModelMetrics struct {
	// Requests is the number of top-level calls received.
	Requests int
	// Successes is the number of calls that ultimately succeeded.
	Successes int
	// Failures is the number of calls that failed after exhausting all fallbacks.
	Failures int
	// FallbackActivations counts how many times a fallback model was tried.
	FallbackActivations int
}

// FallbackModel wraps a primary model and transparently retries with configured
// fallback models on failure. It implements AgnoModelInterface, so it can be used
// anywhere a model is expected.
type FallbackModel struct {
	primary AgnoModelInterface
	config  FallbackConfig

	mu      sync.Mutex
	metrics ModelMetrics
}

// NewFallbackModel wraps primary with the given fallback configuration.
func NewFallbackModel(primary AgnoModelInterface, config FallbackConfig) *FallbackModel {
	return &FallbackModel{primary: primary, config: config}
}

// GetID returns the primary model's ID.
func (f *FallbackModel) GetID() string {
	return f.primary.GetID()
}

// Metrics returns a snapshot of the current metrics.
func (f *FallbackModel) Metrics() ModelMetrics {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.metrics
}

func (f *FallbackModel) recordRequest() {
	f.mu.Lock()
	f.metrics.Requests++
	f.mu.Unlock()
}

func (f *FallbackModel) recordSuccess() {
	f.mu.Lock()
	f.metrics.Successes++
	f.mu.Unlock()
}

func (f *FallbackModel) recordFailure() {
	f.mu.Lock()
	f.metrics.Failures++
	f.mu.Unlock()
}

func (f *FallbackModel) recordFallback() {
	f.mu.Lock()
	f.metrics.FallbackActivations++
	f.mu.Unlock()
}

// chain builds the ordered list of models to try for a given error.
func (f *FallbackModel) chain(err error) []AgnoModelInterface {
	return f.config.selectFallbacks(err)
}

// Invoke tries the primary model, then fallbacks selected by the error type.
func (f *FallbackModel) Invoke(ctx context.Context, messages []Message, options ...Option) (*MessageResponse, error) {
	f.recordRequest()

	resp, err := f.primary.Invoke(ctx, messages, options...)
	if err == nil {
		f.recordSuccess()
		return resp, nil
	}

	lastErr := err
	for _, fb := range f.chain(err) {
		f.recordFallback()
		if f.config.Callback != nil {
			f.config.Callback(f.primary.GetID(), fb.GetID(), lastErr)
		}
		resp, fbErr := fb.Invoke(ctx, messages, options...)
		if fbErr == nil {
			f.recordSuccess()
			return resp, nil
		}
		lastErr = fbErr
	}

	f.recordFailure()
	return nil, lastErr
}

// InvokeStream tries the primary model's stream, then fallbacks on failure.
func (f *FallbackModel) InvokeStream(ctx context.Context, messages []Message, options ...Option) error {
	f.recordRequest()

	err := f.primary.InvokeStream(ctx, messages, options...)
	if err == nil {
		f.recordSuccess()
		return nil
	}

	lastErr := err
	for _, fb := range f.chain(err) {
		f.recordFallback()
		if f.config.Callback != nil {
			f.config.Callback(f.primary.GetID(), fb.GetID(), lastErr)
		}
		if fbErr := fb.InvokeStream(ctx, messages, options...); fbErr == nil {
			f.recordSuccess()
			return nil
		} else {
			lastErr = fbErr
		}
	}

	f.recordFailure()
	return lastErr
}

// AInvoke is the asynchronous version of Invoke.
func (f *FallbackModel) AInvoke(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error) {
	ch := make(chan *MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		resp, err := f.Invoke(ctx, messages, options...)
		if err != nil {
			errChan <- err
			return
		}
		ch <- resp
	}()
	return ch, errChan
}

// AInvokeStream is the asynchronous version of InvokeStream.
func (f *FallbackModel) AInvokeStream(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error) {
	ch := make(chan *MessageResponse)
	errChan := make(chan error)
	go func() {
		defer close(ch)
		defer close(errChan)
		if err := f.InvokeStream(ctx, messages, options...); err != nil {
			errChan <- err
		}
	}()
	return ch, errChan
}

// Ensure FallbackModel satisfies the model interface.
var _ AgnoModelInterface = (*FallbackModel)(nil)

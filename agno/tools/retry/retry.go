package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// BackoffStrategy defines the interface for backoff strategies
type BackoffStrategy interface {
	NextBackoff(attempt int) time.Duration
}

// ExponentialBackoff implements exponential backoff with jitter
type ExponentialBackoff struct {
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	JitterFraction    float64
	mu                sync.Mutex
	rng               *rand.Rand
}

// NewExponentialBackoff creates a new exponential backoff strategy
func NewExponentialBackoff(initial, max time.Duration, multiplier, jitter float64) *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialBackoff:    initial,
		MaxBackoff:        max,
		BackoffMultiplier: multiplier,
		JitterFraction:    jitter,
		rng:               rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NextBackoff calculates the next backoff duration
func (eb *ExponentialBackoff) NextBackoff(attempt int) time.Duration {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Calculate exponential backoff: initialBackoff * multiplier^attempt
	backoff := time.Duration(float64(eb.InitialBackoff) *
		math.Pow(eb.BackoffMultiplier, float64(attempt)))

	// Cap at max backoff
	if backoff > eb.MaxBackoff {
		backoff = eb.MaxBackoff
	}

	// Add jitter: ±jitterFraction%
	jitterAmount := time.Duration(eb.rng.Float64() * float64(backoff) * eb.JitterFraction)
	if eb.rng.Float64() > 0.5 {
		return backoff + jitterAmount
	}
	return backoff - jitterAmount
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts     int
	BackoffStrategy BackoffStrategy
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BackoffStrategy: NewExponentialBackoff(
			100*time.Millisecond,
			30*time.Second,
			2.0,
			0.1,
		),
	}
}

// Result represents the result of an operation
type Result struct {
	Success    bool
	Output     string
	Error      string
	Command    string
	ExitCode   int
	RetryCount int
	Timestamp  time.Time
}

// IsSuccess checks if the result is successful
func (r Result) IsSuccess() bool {
	return r.Success
}

// RetryFunc is a function that can be retried
type RetryFunc func(ctx context.Context) Result

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryFunc) Result {
	var lastResult Result
	lastResult.Timestamp = time.Now()

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		lastResult = fn(ctx)
		lastResult.RetryCount = attempt

		// Check if successful
		if lastResult.IsSuccess() {
			return lastResult
		}

		// Don't retry if this was the last attempt
		if attempt >= config.MaxAttempts-1 {
			break
		}

		// Calculate backoff
		backoff := config.BackoffStrategy.NextBackoff(attempt)

		// Wait for backoff or context cancellation
		select {
		case <-time.After(backoff):
			continue
		case <-ctx.Done():
			lastResult.Error = fmt.Sprintf("Context cancelled after %d attempts: %v", attempt+1, ctx.Err())
			return lastResult
		}
	}

	return lastResult
}

// RetryMetrics tracks retry statistics
type RetryMetrics struct {
	mu                    sync.RWMutex
	OperationName         string
	TotalAttempts         int
	SuccessfulAttempts    int
	FailedAttempts        int
	SuccessAfterRetry     int
	FailedAfterRetries    int
	AverageRetryCount     float64
	LastRetryTimestamp    time.Time
	SuccessRate           float64
	SuccessAfterRetryRate float64
}

// MetricsCollector collects retry metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*RetryMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*RetryMetrics),
	}
}

// RecordResult records the result of a retry operation
func (mc *MetricsCollector) RecordResult(operationName string, result Result) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics, exists := mc.metrics[operationName]
	if !exists {
		metrics = &RetryMetrics{
			OperationName: operationName,
		}
		mc.metrics[operationName] = metrics
	}

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.TotalAttempts++
	metrics.LastRetryTimestamp = time.Now()

	if result.IsSuccess() {
		metrics.SuccessfulAttempts++
		if result.RetryCount > 0 {
			metrics.SuccessAfterRetry++
		}
	} else {
		metrics.FailedAttempts++
		if result.RetryCount > 0 {
			metrics.FailedAfterRetries++
		}
	}

	// Calculate rates
	if metrics.TotalAttempts > 0 {
		metrics.SuccessRate = float64(metrics.SuccessfulAttempts) / float64(metrics.TotalAttempts)
	}
	if metrics.SuccessfulAttempts+metrics.FailedAfterRetries > 0 {
		metrics.SuccessAfterRetryRate = float64(metrics.SuccessAfterRetry) /
			float64(metrics.SuccessfulAttempts+metrics.FailedAfterRetries)
	}
}

// GetMetrics retrieves metrics for an operation
func (mc *MetricsCollector) GetMetrics(operationName string) *RetryMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.metrics[operationName]
	if !exists {
		return nil
	}

	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	// Return a copy
	copy := *metrics
	return &copy
}

// GetAllMetrics retrieves all metrics
func (mc *MetricsCollector) GetAllMetrics() map[string]*RetryMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*RetryMetrics)
	for k, v := range mc.metrics {
		v.mu.RLock()
		copy := *v
		v.mu.RUnlock()
		result[k] = &copy
	}
	return result
}

// Global metrics collector
var globalCollector = NewMetricsCollector()

// GetGlobalCollector returns the global metrics collector
func GetGlobalCollector() *MetricsCollector {
	return globalCollector
}

package main

import (
	"context"
	"fmt"
	"time"

	"agno/tools/retry"
)

func main() {
	fmt.Println("🚀 Q1 2026 - Retry Logic Example")
	fmt.Println("==================================\n")

	// Example 1: Basic Retry with Exponential Backoff
	fmt.Println("📌 Example 1: Basic Retry with Exponential Backoff")
	basicRetryExample()

	fmt.Println("\n")

	// Example 2: Retry with Context Cancellation
	fmt.Println("📌 Example 2: Retry with Context Cancellation")
	contextCancellationExample()

	fmt.Println("\n")

	// Example 3: Retry Metrics Collection
	fmt.Println("📌 Example 3: Retry Metrics Collection")
	metricsExample()
}

// basicRetryExample demonstrates basic retry functionality
func basicRetryExample() {
	attempt := 0

	// Define retry config
	config := retry.RetryConfig{
		MaxAttempts: 3,
		BackoffStrategy: retry.NewExponentialBackoff(
			100*time.Millisecond, // Initial backoff
			5*time.Second,        // Max backoff
			2.0,                  // Multiplier
			0.1,                  // Jitter
		),
	}

	// Define the operation to retry
	operation := func(ctx context.Context) retry.Result {
		attempt++
		fmt.Printf("  Attempt %d at %s\n", attempt, time.Now().Format("15:04:05.000"))

		// Simulate failure on first 2 attempts, success on 3rd
		if attempt < 3 {
			return retry.Result{
				Success: false,
				Output:  "",
				Error:   "simulated connection timeout",
			}
		}

		return retry.Result{
			Success: true,
			Output:  "Operation completed successfully!",
			Error:   "",
		}
	}

	// Execute with retry
	ctx := context.Background()
	result := retry.Retry(ctx, config, operation)

	fmt.Printf("  ✅ Result: Success=%v, Retries=%d\n", result.Success, result.RetryCount)
	if !result.Success {
		fmt.Printf("  ❌ Error: %s\n", result.Error)
	} else {
		fmt.Printf("  📝 Output: %s\n", result.Output)
	}
}

// contextCancellationExample demonstrates context-based cancellation
func contextCancellationExample() {
	attempt := 0

	config := retry.RetryConfig{
		MaxAttempts: 5,
		BackoffStrategy: retry.NewExponentialBackoff(
			500*time.Millisecond,
			10*time.Second,
			2.0,
			0.1,
		),
	}

	// Create context with 1.5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	operation := func(ctx context.Context) retry.Result {
		attempt++
		fmt.Printf("  Attempt %d at %s\n", attempt, time.Now().Format("15:04:05.000"))

		// Always fail (will be cancelled by context)
		return retry.Result{
			Success: false,
			Output:  "",
			Error:   "connection refused",
		}
	}

	// Execute with retry (will be cancelled by context)
	result := retry.Retry(ctx, config, operation)

	fmt.Printf("  ⏱️  Attempts made: %d\n", attempt)
	fmt.Printf("  ❌ Cancelled: %v\n", result.Error != "")
	fmt.Printf("  📝 Error: %s\n", result.Error)
}

// metricsExample demonstrates metrics collection
func metricsExample() {
	collector := retry.GetGlobalCollector()

	// Simulate multiple operations
	for i := 0; i < 3; i++ {
		config := retry.DefaultRetryConfig()

		operation := func(ctx context.Context) retry.Result {
			// Simulate 50% success rate
			if i%2 == 0 {
				return retry.Result{Success: true}
			}
			return retry.Result{Success: false, Error: "failed"}
		}

		result := retry.Retry(context.Background(), config, operation)
		collector.RecordResult("cache.set", result)
	}

	// Display metrics
	metrics := collector.GetMetrics("cache.set")
	if metrics != nil {
		fmt.Printf("  📊 Operation: %s\n", metrics.OperationName)
		fmt.Printf("  🔄 Total Attempts: %d\n", metrics.TotalAttempts)
		fmt.Printf("  ✅ Successful: %d\n", metrics.SuccessfulAttempts)
		fmt.Printf("  ❌ Failed: %d\n", metrics.FailedAttempts)
		fmt.Printf("  📈 Success Rate: %.2f%%\n", metrics.SuccessRate*100)
		fmt.Printf("  🔁 Success After Retry: %d\n", metrics.SuccessAfterRetry)
		fmt.Printf("  📉 Success After Retry Rate: %.2f%%\n", metrics.SuccessAfterRetryRate*100)
	}
}

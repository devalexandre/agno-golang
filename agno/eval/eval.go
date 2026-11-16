// Package eval provides evaluation utilities for agents and models.
//
// It ships three evaluations:
//   - AccuracyEval: runs an agent N times over an input and grades the output
//     against an expected output using a model as judge (score 1..10).
//   - PerformanceEval: measures latency and memory allocation of a function
//     over N iterations, with optional warmup runs.
//   - ReliabilityEval: asserts that the expected tool calls (and optionally
//     their arguments) actually happened during a run.
package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Type identifies the kind of evaluation that produced a result.
type Type string

const (
	// TypeAccuracy identifies an accuracy evaluation.
	TypeAccuracy Type = "accuracy"
	// TypePerformance identifies a performance evaluation.
	TypePerformance Type = "performance"
	// TypeReliability identifies a reliability evaluation.
	TypeReliability Type = "reliability"
)

// Status is the outcome of an evaluation that either passes or fails.
type Status string

const (
	// StatusPassed means the evaluation succeeded.
	StatusPassed Status = "PASSED"
	// StatusFailed means the evaluation failed.
	StatusFailed Status = "FAILED"
)

// AgentRunner is the minimal contract an agent must satisfy to be evaluated.
// *agent.Agent implements it.
type AgentRunner interface {
	Run(input interface{}, opts ...interface{}) (models.RunResponse, error)
}

// Judge is the minimal contract a model must satisfy to be used as a judge.
// Every model implementing models.AgnoModelInterface satisfies it.
type Judge interface {
	Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error)
}

// Stats holds descriptive statistics for a series of measurements.
type Stats struct {
	Count  int     `json:"count"`
	Avg    float64 `json:"avg"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	StdDev float64 `json:"std_dev"`
	Median float64 `json:"median"`
	P95    float64 `json:"p95"`
}

// computeStats returns descriptive statistics for the given values.
// It returns a zero valued Stats when values is empty.
func computeStats(values []float64) Stats {
	if len(values) == 0 {
		return Stats{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	avg := sum / float64(len(sorted))

	stdDev := 0.0
	if len(sorted) > 1 {
		var variance float64
		for _, v := range sorted {
			variance += (v - avg) * (v - avg)
		}
		// Sample standard deviation, matching the Python implementation.
		stdDev = math.Sqrt(variance / float64(len(sorted)-1))
	}

	return Stats{
		Count:  len(sorted),
		Avg:    avg,
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		StdDev: stdDev,
		Median: median(sorted),
		P95:    percentile(sorted, 0.95),
	}
}

// median returns the median of an already sorted slice.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	mid := n / 2
	if n%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// percentile returns the linear interpolated percentile of an already sorted slice.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	pos := p * float64(n-1)
	lower := int(math.Floor(pos))
	upper := int(math.Ceil(pos))
	if lower == upper {
		return sorted[lower]
	}
	weight := pos - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// storedResult is the envelope used when persisting an evaluation result to disk.
type storedResult struct {
	Name   string      `json:"name,omitempty"`
	EvalID string      `json:"eval_id,omitempty"`
	Type   Type        `json:"eval_type"`
	Result interface{} `json:"result"`
}

// storeResultInFile writes the given result as JSON to filePath, creating parent directories.
func storeResultInFile(filePath, name, evalID string, evalType Type, result interface{}) error {
	if filePath == "" {
		return nil
	}
	if dir := filepath.Dir(filePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory for results: %w", err)
		}
	}

	data, err := json.MarshalIndent(storedResult{
		Name:   name,
		EvalID: evalID,
		Type:   evalType,
		Result: result,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize evaluation result: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write evaluation result: %w", err)
	}
	return nil
}

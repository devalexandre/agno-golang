package eval

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPerformanceEvalRun(t *testing.T) {
	calls := 0
	evaluation := &PerformanceEval{
		Name: "allocation",
		Func: func() error {
			calls++
			buffer := make([]byte, 1024*512)
			buffer[0] = 1
			return nil
		},
		MeasureRuntime: true,
		MeasureMemory:  true,
		WarmupRuns:     2,
		NumIterations:  5,
	}

	result, err := evaluation.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 warmup runs + 5 run time runs + 5 memory runs.
	if calls != 12 {
		t.Fatalf("expected 12 calls, got %d", calls)
	}
	if len(result.RunTimes) != 5 || len(result.MemoryUsages) != 5 {
		t.Fatalf("expected 5 measurements each, got %d and %d", len(result.RunTimes), len(result.MemoryUsages))
	}
	if result.RunTimeStats.Count != 5 || result.MemoryStats.Count != 5 {
		t.Fatalf("unexpected stats counts: %+v %+v", result.RunTimeStats, result.MemoryStats)
	}
	if result.RunTimeStats.Min > result.RunTimeStats.Avg || result.RunTimeStats.Avg > result.RunTimeStats.Max {
		t.Fatalf("inconsistent run time stats: %+v", result.RunTimeStats)
	}
	if result.MemoryStats.Avg <= 0 {
		t.Fatalf("expected a positive memory usage, got %f", result.MemoryStats.Avg)
	}
	if _, err := result.JSON(); err != nil {
		t.Fatalf("failed to serialize the result: %v", err)
	}
}

func TestPerformanceEvalRuntimeOnly(t *testing.T) {
	evaluation := &PerformanceEval{
		Func:           func() error { return nil },
		MeasureRuntime: true,
		NumIterations:  3,
	}

	result, err := evaluation.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.RunTimes) != 3 {
		t.Fatalf("expected 3 run times, got %d", len(result.RunTimes))
	}
	if len(result.MemoryUsages) != 0 {
		t.Fatalf("expected no memory measurements, got %d", len(result.MemoryUsages))
	}
}

func TestPerformanceEvalErrors(t *testing.T) {
	if _, err := (&PerformanceEval{MeasureRuntime: true}).Run(); err == nil {
		t.Fatal("expected an error when no function is given")
	}

	noMeasurement := &PerformanceEval{Func: func() error { return nil }}
	if _, err := noMeasurement.Run(); err == nil {
		t.Fatal("expected an error when no measurement is enabled")
	}

	failing := &PerformanceEval{
		Func:           func() error { return errors.New("boom") },
		MeasureRuntime: true,
		NumIterations:  1,
	}
	if _, err := failing.Run(); err == nil {
		t.Fatal("expected an error when the function fails")
	}
}

func TestNewPerformanceEvalDefaults(t *testing.T) {
	evaluation := NewPerformanceEval("defaults", func() error { return nil })

	if !evaluation.MeasureRuntime || !evaluation.MeasureMemory {
		t.Fatal("expected both measurements to be enabled by default")
	}
	if evaluation.NumIterations != 10 || evaluation.WarmupRuns != 1 {
		t.Fatalf("unexpected defaults: %d iterations, %d warmup runs", evaluation.NumIterations, evaluation.WarmupRuns)
	}
}

func TestPerformanceResultPrint(t *testing.T) {
	result := &PerformanceResult{RunTimes: []float64{0.1, 0.2}, MemoryUsages: []float64{1, 2}}
	result.ComputeStats()

	var summary, results bytes.Buffer
	result.PrintSummary(&summary)
	result.PrintResults(&results)

	if !strings.Contains(summary.String(), "Run time (seconds)") {
		t.Fatalf("unexpected summary: %s", summary.String())
	}
	if !strings.Contains(results.String(), "Run 2") {
		t.Fatalf("unexpected results: %s", results.String())
	}
}

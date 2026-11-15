package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
)

// bytesPerMiB is used to convert allocated bytes into MiB.
const bytesPerMiB = 1024 * 1024

// PerformanceResult holds run time and memory usage statistics of a performance evaluation.
type PerformanceResult struct {
	// RunTimes holds the duration of every measured run, in seconds.
	RunTimes []float64 `json:"run_times"`
	// RunTimeStats holds the statistics computed over RunTimes.
	RunTimeStats Stats `json:"run_time_stats"`

	// MemoryUsages holds the memory allocated by every measured run, in MiB.
	MemoryUsages []float64 `json:"memory_usages"`
	// MemoryStats holds the statistics computed over MemoryUsages.
	MemoryStats Stats `json:"memory_stats"`
}

// ComputeStats recomputes the run time and memory statistics from the collected measurements.
func (r *PerformanceResult) ComputeStats() {
	r.RunTimeStats = computeStats(r.RunTimes)
	r.MemoryStats = computeStats(r.MemoryUsages)
}

// JSON serializes the result.
func (r *PerformanceResult) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// PrintSummary writes the aggregated statistics to w. When w is nil, os.Stdout is used.
func (r *PerformanceResult) PrintSummary(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "=== Performance Evaluation Summary ===")
	if len(r.RunTimes) > 0 {
		fmt.Fprintln(w, "Run time (seconds)")
		printStats(w, r.RunTimeStats)
	}
	if len(r.MemoryUsages) > 0 {
		fmt.Fprintln(w, "Memory (MiB)")
		printStats(w, r.MemoryStats)
	}
}

// PrintResults writes every individual measurement to w. When w is nil, os.Stdout is used.
func (r *PerformanceResult) PrintResults(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "=== Performance Evaluation Runs ===")

	total := len(r.RunTimes)
	if len(r.MemoryUsages) > total {
		total = len(r.MemoryUsages)
	}

	for i := 0; i < total; i++ {
		fmt.Fprintf(w, "Run %d", i+1)
		if i < len(r.RunTimes) {
			fmt.Fprintf(w, " | time: %.6fs", r.RunTimes[i])
		}
		if i < len(r.MemoryUsages) {
			fmt.Fprintf(w, " | memory: %.6f MiB", r.MemoryUsages[i])
		}
		fmt.Fprintln(w)
	}
}

// printStats writes a single Stats block to w.
func printStats(w io.Writer, stats Stats) {
	fmt.Fprintf(w, "  Average   : %.6f\n", stats.Avg)
	fmt.Fprintf(w, "  Minimum   : %.6f\n", stats.Min)
	fmt.Fprintf(w, "  Maximum   : %.6f\n", stats.Max)
	fmt.Fprintf(w, "  Std Dev   : %.6f\n", stats.StdDev)
	fmt.Fprintf(w, "  Median    : %.6f\n", stats.Median)
	fmt.Fprintf(w, "  95th %%ile : %.6f\n", stats.P95)
}

// PerformanceEval measures the run time and the memory allocated by a function.
//
// Warmup runs are executed first and excluded from the statistics, so that
// one-off initialization costs do not pollute the measurements.
type PerformanceEval struct {
	// Name of the evaluation.
	Name string
	// EvalID is a unique identifier for the evaluation. Generated when empty.
	EvalID string

	// Func is the function under evaluation.
	Func func() error

	// MeasureRuntime enables the run time measurements.
	MeasureRuntime bool
	// MeasureMemory enables the memory allocation measurements.
	MeasureMemory bool

	// WarmupRuns is the number of runs executed before measuring.
	WarmupRuns int
	// NumIterations is the number of measured runs. Defaults to 1.
	NumIterations int

	// PrintSummary prints the aggregated statistics after the run.
	PrintSummary bool
	// PrintResults prints every individual measurement after the run.
	PrintResults bool
	// Writer used when printing. Defaults to os.Stdout.
	Writer io.Writer
	// FilePathToSaveResults persists the result as JSON when set.
	FilePathToSaveResults string

	// Result of the last run.
	Result *PerformanceResult
}

// NewPerformanceEval returns a PerformanceEval measuring both run time and memory.
func NewPerformanceEval(name string, fn func() error) *PerformanceEval {
	return &PerformanceEval{
		Name:           name,
		Func:           fn,
		MeasureRuntime: true,
		MeasureMemory:  true,
		WarmupRuns:     1,
		NumIterations:  10,
	}
}

// evalID returns the evaluation ID, generating one when needed.
func (e *PerformanceEval) evalID() string {
	if e.EvalID == "" {
		e.EvalID = uuid.NewString()
	}
	return e.EvalID
}

// iterations returns the configured number of iterations, defaulting to 1.
func (e *PerformanceEval) iterations() int {
	if e.NumIterations <= 0 {
		return 1
	}
	return e.NumIterations
}

// measureTime runs the function once and returns its duration in seconds.
func (e *PerformanceEval) measureTime() (float64, error) {
	start := time.Now()
	if err := e.Func(); err != nil {
		return 0, err
	}
	return time.Since(start).Seconds(), nil
}

// measureMemory runs the function once and returns the memory it allocated, in MiB.
func (e *PerformanceEval) measureMemory() (float64, error) {
	var before, after runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&before)

	if err := e.Func(); err != nil {
		return 0, err
	}

	runtime.ReadMemStats(&after)

	allocated := after.TotalAlloc - before.TotalAlloc
	return float64(allocated) / bytesPerMiB, nil
}

// Run executes the warmup runs and then measures the function over NumIterations.
func (e *PerformanceEval) Run() (*PerformanceResult, error) {
	if e.Func == nil {
		return nil, fmt.Errorf("eval: a function is required to run the performance evaluation")
	}
	if !e.MeasureRuntime && !e.MeasureMemory {
		return nil, fmt.Errorf("eval: enable at least one of MeasureRuntime or MeasureMemory")
	}

	for i := 0; i < e.WarmupRuns; i++ {
		if err := e.Func(); err != nil {
			return nil, fmt.Errorf("eval: warmup run %d failed: %w", i+1, err)
		}
	}

	iterations := e.iterations()
	result := &PerformanceResult{}

	if e.MeasureRuntime {
		result.RunTimes = make([]float64, 0, iterations)
		for i := 0; i < iterations; i++ {
			elapsed, err := e.measureTime()
			if err != nil {
				return nil, fmt.Errorf("eval: run time measurement %d failed: %w", i+1, err)
			}
			result.RunTimes = append(result.RunTimes, elapsed)
		}
	}

	if e.MeasureMemory {
		result.MemoryUsages = make([]float64, 0, iterations)
		for i := 0; i < iterations; i++ {
			usage, err := e.measureMemory()
			if err != nil {
				return nil, fmt.Errorf("eval: memory measurement %d failed: %w", i+1, err)
			}
			result.MemoryUsages = append(result.MemoryUsages, usage)
		}
	}

	result.ComputeStats()
	e.Result = result

	if e.FilePathToSaveResults != "" {
		if err := storeResultInFile(e.FilePathToSaveResults, e.Name, e.evalID(), TypePerformance, result); err != nil {
			return nil, err
		}
	}

	if e.PrintResults {
		result.PrintResults(e.Writer)
	}
	if e.PrintSummary {
		result.PrintSummary(e.Writer)
	}

	return result, nil
}

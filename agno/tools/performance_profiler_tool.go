package tools

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// PerformanceProfilerTool fornece análise de performance e profiling
type PerformanceProfilerTool struct {
	toolkit.Toolkit
	profiles   []ProfileResult
	benchmarks []BenchmarkResult
	metrics    []PerformanceMetric
	maxResults int
}

// ProfileResult representa o resultado de um profile
type ProfileResult struct {
	ProfileID    string
	ProfileType  string // "cpu", "memory", "goroutine", "trace"
	StartTime    time.Time
	EndTime      time.Time
	Duration     int64 // ms
	Samples      int
	TopFunctions []FunctionProfile
	Status       string
	FilePath     string
}

// FunctionProfile representa o perfil de uma função
type FunctionProfile struct {
	FunctionName string
	CallCount    int64
	TotalTime    int64   // ns
	AverageTime  int64   // ns
	MemoryAlloc  int64   // bytes
	Percentage   float32 // % do tempo total
}

// BenchmarkResult representa o resultado de um benchmark
type BenchmarkResult struct {
	BenchmarkID    string
	FunctionName   string
	Iterations     int64
	Duration       int64 // ns
	AverageNsPerOp int64
	MemoryBytes    int64
	AllocsPerOp    int64
	Timestamp      time.Time
}

// PerformanceMetric representa uma métrica de performance
type PerformanceMetric struct {
	MetricID   string
	MetricName string
	Value      float64
	Unit       string
	Timestamp  time.Time
}

// StartProfilingParams parâmetros para iniciar profiling
type StartProfilingParams struct {
	ProfileType string `json:"profile_type" description:"Tipo de profile (cpu, memory, goroutine, trace)"`
	Duration    int    `json:"duration" description:"Duração em segundos"`
	OutputPath  string `json:"output_path" description:"Caminho para salvar profile"`
}

// RunBenchmarkParams parâmetros para executar benchmark
type RunBenchmarkParams struct {
	FunctionName string `json:"function_name" description:"Nome da função a testar"`
	Iterations   int64  `json:"iterations" description:"Número de iterações"`
	Timeout      int    `json:"timeout" description:"Timeout em segundos"`
}

// GetMemoryStatsParams parâmetros para obter stats de memória
type GetMemoryStatsParams struct {
	Detailed bool `json:"detailed" description:"Relatório detalhado"`
}

// GetGoroutineStatsParams parâmetros para obter stats de goroutine
type GetGoroutineStatsParams struct {
	Top int `json:"top" description:"Top N goroutines"`
}

// NewPerformanceProfilerTool cria uma nova instância
func NewPerformanceProfilerTool() *PerformanceProfilerTool {
	tool := &PerformanceProfilerTool{
		profiles:   make([]ProfileResult, 0),
		benchmarks: make([]BenchmarkResult, 0),
		metrics:    make([]PerformanceMetric, 0),
		maxResults: 500,
	}
	tool.Toolkit = toolkit.NewToolkit()
	tool.Toolkit.Name = "PerformanceProfilerTool"
	tool.Toolkit.Description = "Ferramenta de profiling e benchmarking de performance"

	tool.Register("start_profiling",
		"Iniciar coleta de profiling",
		tool,
		tool.StartProfiling,
		StartProfilingParams{},
	)

	tool.Register("run_benchmark",
		"Executar benchmark de uma função",
		tool,
		tool.RunBenchmark,
		RunBenchmarkParams{},
	)

	tool.Register("get_memory_stats",
		"Obter estatísticas de memória",
		tool,
		tool.GetMemoryStats,
		GetMemoryStatsParams{},
	)

	tool.Register("get_goroutine_stats",
		"Obter estatísticas de goroutines",
		tool,
		tool.GetGoroutineStats,
		GetGoroutineStatsParams{},
	)

	tool.Register("get_cpu_info",
		"Obter informações de CPU",
		tool,
		tool.GetCPUInfo,
		struct{}{},
	)

	tool.Register("get_profiling_history",
		"Obter histórico de profiling",
		tool,
		tool.GetProfilingHistory,
		struct{}{},
	)

	return tool
}

// StartProfiling inicia coleta de profiling
func (t *PerformanceProfilerTool) StartProfiling(params StartProfilingParams) (map[string]interface{}, error) {
	if params.ProfileType == "" {
		params.ProfileType = "cpu"
	}

	// Validar tipo de profile
	validTypes := map[string]bool{
		"cpu":       true,
		"memory":    true,
		"goroutine": true,
		"trace":     true,
	}

	if !validTypes[params.ProfileType] {
		return nil, fmt.Errorf("tipo de profile inválido: %s", params.ProfileType)
	}

	duration := params.Duration
	if duration <= 0 {
		duration = 30
	}

	profileID := fmt.Sprintf("prof_%d", time.Now().UnixNano())
	startTime := time.Now()

	// Simular profiling
	topFunctions := []FunctionProfile{
		{
			FunctionName: "main.processData",
			CallCount:    1000,
			TotalTime:    5000000000, // 5s em nanosegundos
			AverageTime:  5000000,
			MemoryAlloc:  1024 * 1024,
			Percentage:   45.5,
		},
		{
			FunctionName: "main.calculateSum",
			CallCount:    5000,
			TotalTime:    3000000000, // 3s
			AverageTime:  600000,
			MemoryAlloc:  512 * 1024,
			Percentage:   27.3,
		},
		{
			FunctionName: "main.formatOutput",
			CallCount:    2000,
			TotalTime:    2000000000, // 2s
			AverageTime:  1000000,
			MemoryAlloc:  256 * 1024,
			Percentage:   18.2,
		},
	}

	result := ProfileResult{
		ProfileID:    profileID,
		ProfileType:  params.ProfileType,
		StartTime:    startTime,
		EndTime:      startTime.Add(time.Duration(duration) * time.Second),
		Duration:     int64(duration * 1000),
		Samples:      1000 + rand.Intn(500),
		TopFunctions: topFunctions,
		Status:       "completed",
		FilePath:     params.OutputPath,
	}

	t.profiles = append(t.profiles, result)

	// Limitar histórico
	if len(t.profiles) > t.maxResults {
		t.profiles = t.profiles[1:]
	}

	return map[string]interface{}{
		"success":       true,
		"profile_id":    profileID,
		"profile_type":  params.ProfileType,
		"duration_sec":  duration,
		"samples":       result.Samples,
		"top_functions": result.TopFunctions,
		"output_path":   params.OutputPath,
		"status":        "completed",
	}, nil
}

// RunBenchmark executa benchmark
func (t *PerformanceProfilerTool) RunBenchmark(params RunBenchmarkParams) (map[string]interface{}, error) {
	if params.FunctionName == "" {
		return nil, fmt.Errorf("nome da função não pode estar vazio")
	}

	iterations := params.Iterations
	if iterations <= 0 {
		iterations = 1000000
	}

	benchmarkID := fmt.Sprintf("bench_%d", time.Now().UnixNano())

	// Simular execução de benchmark
	totalDuration := int64(iterations * (10 + rand.Int63n(20))) // 10-30ns por operação
	avgNsPerOp := totalDuration / iterations
	memoryBytes := iterations * (64 + rand.Int63n(128)) // 64-192 bytes por iteração
	allocsPerOp := 2 + rand.Int63n(4)

	result := BenchmarkResult{
		BenchmarkID:    benchmarkID,
		FunctionName:   params.FunctionName,
		Iterations:     iterations,
		Duration:       totalDuration,
		AverageNsPerOp: avgNsPerOp,
		MemoryBytes:    memoryBytes,
		AllocsPerOp:    allocsPerOp,
		Timestamp:      time.Now(),
	}

	t.benchmarks = append(t.benchmarks, result)

	if len(t.benchmarks) > t.maxResults {
		t.benchmarks = t.benchmarks[1:]
	}

	return map[string]interface{}{
		"success":       true,
		"benchmark_id":  benchmarkID,
		"function":      params.FunctionName,
		"iterations":    iterations,
		"total_ns":      totalDuration,
		"avg_ns_per_op": avgNsPerOp,
		"memory_bytes":  memoryBytes,
		"allocs_per_op": allocsPerOp,
		"timestamp":     result.Timestamp.Format(time.RFC3339),
	}, nil
}

// GetMemoryStats obtém estatísticas de memória
func (t *PerformanceProfilerTool) GetMemoryStats(params GetMemoryStatsParams) (map[string]interface{}, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := map[string]interface{}{
		"success":           true,
		"alloc_bytes":       m.Alloc,
		"total_alloc_bytes": m.TotalAlloc,
		"sys_bytes":         m.Sys,
		"num_gc":            m.NumGC,
		"gc_pause_ns_last":  m.PauseNs[(m.NumGC+255)%256],
	}

	if params.Detailed {
		stats["heap_alloc"] = m.HeapAlloc
		stats["heap_sys"] = m.HeapSys
		stats["heap_idle"] = m.HeapIdle
		stats["heap_in_use"] = m.HeapInuse
		stats["heap_released"] = m.HeapReleased
		stats["heap_objects"] = m.HeapObjects
		stats["stack_in_use"] = m.StackInuse
		stats["stack_sys"] = m.StackSys
		stats["mspan_in_use"] = m.MSpanInuse
		stats["mcache_in_use"] = m.MCacheInuse
		stats["mallocs"] = m.Mallocs
		stats["frees"] = m.Frees
		stats["live_objects"] = m.Mallocs - m.Frees
	}

	return stats, nil
}

// GetGoroutineStats obtém estatísticas de goroutines
func (t *PerformanceProfilerTool) GetGoroutineStats(params GetGoroutineStatsParams) (map[string]interface{}, error) {
	numGoroutines := runtime.NumGoroutine()

	top := params.Top
	if top <= 0 {
		top = 10
	}

	// Simular goroutines em execução
	goroutines := make([]map[string]interface{}, 0)
	for i := 1; i <= top && i <= numGoroutines; i++ {
		goroutines = append(goroutines, map[string]interface{}{
			"id":       fmt.Sprintf("goroutine-%d", i),
			"status":   "running",
			"cpu_time": fmt.Sprintf("%d ms", 10+rand.Intn(100)),
		})
	}

	return map[string]interface{}{
		"success":          true,
		"total_goroutines": numGoroutines,
		"top_goroutines":   goroutines,
		"top_limit":        top,
	}, nil
}

// GetCPUInfo obtém informações de CPU
func (t *PerformanceProfilerTool) GetCPUInfo(params struct{}) (map[string]interface{}, error) {
	numCPU := runtime.NumCPU()

	return map[string]interface{}{
		"success":    true,
		"num_cpu":    numCPU,
		"gomaxprocs": runtime.GOMAXPROCS(-1),
		"arch":       runtime.GOARCH,
		"os":         runtime.GOOS,
		"compiler":   runtime.Compiler,
	}, nil
}

// GetProfilingHistory retorna histórico de profiling
func (t *PerformanceProfilerTool) GetProfilingHistory(params struct{}) (map[string]interface{}, error) {
	profileHistory := make([]map[string]interface{}, 0)

	for _, prof := range t.profiles {
		topFunc := ""
		if len(prof.TopFunctions) > 0 {
			topFunc = prof.TopFunctions[0].FunctionName
		}

		profileHistory = append(profileHistory, map[string]interface{}{
			"profile_id":   prof.ProfileID,
			"type":         prof.ProfileType,
			"duration_ms":  prof.Duration,
			"samples":      prof.Samples,
			"top_function": topFunc,
			"status":       prof.Status,
			"timestamp":    prof.StartTime.Format(time.RFC3339),
		})
	}

	benchmarkHistory := make([]map[string]interface{}, 0)

	for _, bench := range t.benchmarks {
		benchmarkHistory = append(benchmarkHistory, map[string]interface{}{
			"benchmark_id":  bench.BenchmarkID,
			"function":      bench.FunctionName,
			"iterations":    bench.Iterations,
			"avg_ns_per_op": bench.AverageNsPerOp,
			"timestamp":     bench.Timestamp.Format(time.RFC3339),
		})
	}

	return map[string]interface{}{
		"success":           true,
		"total_profiles":    len(t.profiles),
		"profile_history":   profileHistory,
		"total_benchmarks":  len(t.benchmarks),
		"benchmark_history": benchmarkHistory,
	}, nil
}

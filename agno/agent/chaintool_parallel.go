package agent

import (
	"context"
	"sync"
	"time"
)

type ParallelExecutionStrategy string

const (
	StrategySequential ParallelExecutionStrategy = "sequential"
	StrategyParallel   ParallelExecutionStrategy = "parallel"
	StrategyWaitAny    ParallelExecutionStrategy = "wait_any"
	StrategyWaitAll    ParallelExecutionStrategy = "wait_all"
	StrategyPipelined  ParallelExecutionStrategy = "pipelined"
	StrategyFanOut     ParallelExecutionStrategy = "fan_out"
)

type ChainToolParallelConfig struct {
	Strategy           ParallelExecutionStrategy
	MaxConcurrency     int
	Timeout            time.Duration
	FailFast           bool
	PreserveOrder      bool
	EnableMetrics      bool
	ContextPropagation bool
	DependencyGraph    map[string][]string
	BatchSize          int
}

type ExecutionMetrics struct {
	ToolName         string
	ExecutionTime    time.Duration
	WaitTime         time.Duration
	StartTime        time.Time
	EndTime          time.Time
	ThreadID         int
	MemoryAllocated  int64
	Success          bool
	Error            error
	CacheHit         bool
	Retries          int
	DependencyWaitMs int64
}

type ParallelExecutionResult struct {
	ToolName string
	Result   interface{}
	Error    error
	Metrics  ExecutionMetrics
	Order    int
}

type ChainToolParallelExecutor interface {
	ExecuteParallel(ctx context.Context, tools []interface{}, inputs []interface{}) ([]ParallelExecutionResult, error)
	ExecuteWithDependencies(ctx context.Context, toolDeps map[string][]string) (map[string]interface{}, error)
	GetMetrics() map[string]ExecutionMetrics
}

type DefaultParallelExecutor struct {
	config    *ChainToolParallelConfig
	semaphore chan struct{}
	metrics   map[string]ExecutionMetrics
	mu        sync.RWMutex
}

func NewDefaultParallelExecutor(config *ChainToolParallelConfig) *DefaultParallelExecutor {
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 4
	}

	executor := &DefaultParallelExecutor{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrency),
		metrics:   make(map[string]ExecutionMetrics),
	}

	return executor
}

func (e *DefaultParallelExecutor) ExecuteParallel(ctx context.Context, tools []interface{}, inputs []interface{}) ([]ParallelExecutionResult, error) {
	if e.config.Strategy == StrategySequential {
		return e.executeSequential(ctx, tools, inputs)
	}

	results := make([]ParallelExecutionResult, len(tools))
	var wg sync.WaitGroup
	errCh := make(chan error, len(tools))

	for i := range tools {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			e.semaphore <- struct{}{}
			defer func() { <-e.semaphore }()

			result := e.executeWithMetrics(ctx, tools[index], inputs[index])
			results[index] = result

			if result.Error != nil && e.config.FailFast {
				errCh <- result.Error
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

func (e *DefaultParallelExecutor) ExecuteWithDependencies(ctx context.Context, toolDeps map[string][]string) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, len(toolDeps))

	inProgress := make(map[string]bool)
	var inProgressMu sync.Mutex

	for toolName, deps := range toolDeps {
		wg.Add(1)
		go func(name string, dependencies []string) {
			defer wg.Done()

			for _, dep := range dependencies {
				for {
					inProgressMu.Lock()
					if !inProgress[dep] {
						inProgressMu.Unlock()
						break
					}
					inProgressMu.Unlock()
					time.Sleep(10 * time.Millisecond)
				}
			}

			inProgressMu.Lock()
			inProgress[name] = true
			inProgressMu.Unlock()

			e.semaphore <- struct{}{}
			defer func() {
				<-e.semaphore
				inProgressMu.Lock()
				inProgress[name] = false
				inProgressMu.Unlock()
			}()

			result := e.executeWithMetrics(ctx, name, nil)

			mu.Lock()
			results[name] = result.Result
			if result.Error != nil {
				errCh <- result.Error
			}
			mu.Unlock()
		}(toolName, deps)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && e.config.FailFast {
			return results, err
		}
	}

	return results, nil
}

func (e *DefaultParallelExecutor) executeSequential(ctx context.Context, tools []interface{}, inputs []interface{}) ([]ParallelExecutionResult, error) {
	results := make([]ParallelExecutionResult, len(tools))

	for i := range tools {
		results[i] = e.executeWithMetrics(ctx, tools[i], inputs[i])
		if results[i].Error != nil && e.config.FailFast {
			return results, results[i].Error
		}
	}

	return results, nil
}

func (e *DefaultParallelExecutor) executeWithMetrics(ctx context.Context, tool interface{}, input interface{}) ParallelExecutionResult {
	metrics := ExecutionMetrics{
		StartTime: time.Now(),
	}

	result := ParallelExecutionResult{
		Order:   0,
		Metrics: metrics,
	}

	if e.config.Timeout > 0 {
		ctx, _ = context.WithTimeout(ctx, e.config.Timeout)
	}

	result.Metrics.EndTime = time.Now()
	result.Metrics.ExecutionTime = time.Since(metrics.StartTime)

	if e.config.EnableMetrics {
		e.mu.Lock()
		if toolName, ok := tool.(string); ok {
			e.metrics[toolName] = result.Metrics
		}
		e.mu.Unlock()
	}

	return result
}

func (e *DefaultParallelExecutor) GetMetrics() map[string]ExecutionMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metricsCopy := make(map[string]ExecutionMetrics)
	for k, v := range e.metrics {
		metricsCopy[k] = v
	}

	return metricsCopy
}

type PipelinedExecutor struct {
	stages   [][]interface{}
	executor *DefaultParallelExecutor
}

func NewPipelinedExecutor(config *ChainToolParallelConfig) *PipelinedExecutor {
	return &PipelinedExecutor{
		executor: NewDefaultParallelExecutor(config),
	}
}

func (pe *PipelinedExecutor) AddStage(tools []interface{}) {
	pe.stages = append(pe.stages, tools)
}

func (pe *PipelinedExecutor) ExecutePipeline(ctx context.Context) ([]interface{}, error) {
	var results []interface{}

	for stageIdx, stageTool := range pe.stages {
		stageResults, err := pe.executor.ExecuteParallel(ctx, stageTool, make([]interface{}, len(stageTool)))
		if err != nil {
			return nil, err
		}

		for _, r := range stageResults {
			results = append(results, r.Result)
		}

		if stageIdx < len(pe.stages)-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return results, nil
}

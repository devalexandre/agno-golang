package v2

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Parallel represents a construct that executes multiple steps concurrently
type Parallel struct {
	Name        string
	Description string
	Steps       []interface{} // Can contain Steps, other Parallels, Loops, etc.

	// Configuration
	MaxConcurrency  int
	FailFast        bool // Stop all if one fails
	WaitForAll      bool // Wait for all to complete even if one fails
	CombineOutputs  bool // Combine outputs into a single output
	TimeoutSeconds  int
	ContinueOnError bool // Continue executing other steps even if some fail

	// Internal state
	outputs map[string]*StepOutput
	mu      sync.Mutex
}

// NewParallel creates a new Parallel instance
func NewParallel(options ...ParallelOption) *Parallel {
	p := &Parallel{
		MaxConcurrency:  10, // Default max concurrency
		WaitForAll:      true,
		CombineOutputs:  true,
		ContinueOnError: false,
		outputs:         make(map[string]*StepOutput),
	}

	for _, opt := range options {
		opt(p)
	}

	return p
}

// ParallelOption is a functional option for configuring a Parallel
type ParallelOption func(*Parallel)

// WithParallelName sets the parallel execution name
func WithParallelName(name string) ParallelOption {
	return func(p *Parallel) {
		p.Name = name
	}
}

// WithParallelDescription sets the parallel execution description
func WithParallelDescription(desc string) ParallelOption {
	return func(p *Parallel) {
		p.Description = desc
	}
}

// WithParallelSteps sets the steps to execute in parallel
func WithParallelSteps(steps ...interface{}) ParallelOption {
	return func(p *Parallel) {
		p.Steps = steps
	}
}

// WithMaxConcurrency sets the maximum number of concurrent executions
func WithMaxConcurrency(max int) ParallelOption {
	return func(p *Parallel) {
		p.MaxConcurrency = max
	}
}

// WithFailFast enables fail-fast mode
func WithFailFast(failFast bool) ParallelOption {
	return func(p *Parallel) {
		p.FailFast = failFast
		if failFast {
			p.WaitForAll = false
		}
	}
}

// WithWaitForAll enables waiting for all steps to complete
func WithWaitForAll(wait bool) ParallelOption {
	return func(p *Parallel) {
		p.WaitForAll = wait
	}
}

// WithCombineOutputs enables combining outputs from all parallel executions
func WithCombineOutputs(combine bool) ParallelOption {
	return func(p *Parallel) {
		p.CombineOutputs = combine
	}
}

// WithParallelTimeout sets the timeout for parallel execution
func WithParallelTimeout(seconds int) ParallelOption {
	return func(p *Parallel) {
		p.TimeoutSeconds = seconds
	}
}

// WithContinueOnError enables continuing execution even if some steps fail
func WithContinueOnError(continueOnError bool) ParallelOption {
	return func(p *Parallel) {
		p.ContinueOnError = continueOnError
	}
}

// Execute runs all steps in parallel with the given input
func (p *Parallel) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	if len(p.Steps) == 0 {
		return &StepOutput{
			StepName:     p.Name,
			ExecutorType: "parallel",
			Event:        string(ParallelExecutionCompletedEvent),
			Metadata: map[string]interface{}{
				"message": "no steps to execute",
			},
		}, nil
	}

	// Apply timeout if configured
	if p.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(p.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Reset outputs
	p.mu.Lock()
	p.outputs = make(map[string]*StepOutput)
	p.mu.Unlock()

	startTime := time.Now()

	// Create channels for coordination
	type result struct {
		name   string
		output *StepOutput
		err    error
	}

	resultChan := make(chan result, len(p.Steps))
	errorChan := make(chan error, len(p.Steps))
	semaphore := make(chan struct{}, p.MaxConcurrency)

	// Create a context that can be cancelled if fail-fast is enabled
	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()

	// WaitGroup to track all goroutines
	var wg sync.WaitGroup

	// Launch goroutines for each step
	for i, item := range p.Steps {
		wg.Add(1)
		stepName := p.getStepName(item, i)

		go func(idx int, stepItem interface{}, name string) {
			defer wg.Done()

			// Acquire semaphore (limit concurrency)
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-execCtx.Done():
				return
			}

			// Check if we should continue (for fail-fast mode)
			select {
			case <-execCtx.Done():
				return
			default:
			}

			// Create a copy of input for this step
			stepInput := &StepInput{
				Message:             input.Message,
				PreviousStepContent: input.PreviousStepContent,
				AdditionalData:      input.AdditionalData,
				Images:              input.Images,
				Videos:              input.Videos,
				Audio:               input.Audio,
				PreviousStepOutputs: make(map[string]*StepOutput),
			}

			// Copy previous step outputs
			if input.PreviousStepOutputs != nil {
				for k, v := range input.PreviousStepOutputs {
					stepInput.PreviousStepOutputs[k] = v
				}
			}

			// Execute the step
			output, err := p.executeStep(execCtx, stepItem, stepInput)

			// Ensure output has the step name
			if output != nil && output.StepName == "" {
				output.StepName = name
			}

			if err != nil {
				if p.FailFast && !p.ContinueOnError {
					cancelExec() // Cancel all other executions
				}
				errorChan <- fmt.Errorf("parallel step '%s' failed: %w", name, err)
				if !p.ContinueOnError {
					return
				}
			}

			// Send result
			resultChan <- result{
				name:   name,
				output: output,
				err:    err,
			}

		}(i, item, stepName)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	var errors []error
	successCount := 0
	failureCount := 0

	for r := range resultChan {
		if r.err != nil {
			failureCount++
			if !p.ContinueOnError {
				errors = append(errors, r.err)
			}
		} else {
			successCount++
			p.mu.Lock()
			p.outputs[r.name] = r.output
			p.mu.Unlock()
		}
	}

	// Check for errors
	if len(errors) > 0 && !p.ContinueOnError {
		return nil, fmt.Errorf("parallel execution failed with %d errors: %v", len(errors), errors[0])
	}

	endTime := time.Now()

	// Create final output
	output := &StepOutput{
		StepName:     p.Name,
		ExecutorType: "parallel",
		Event:        string(ParallelExecutionCompletedEvent),
		Metadata: map[string]interface{}{
			"duration_ms":     endTime.Sub(startTime).Milliseconds(),
			"total_steps":     len(p.Steps),
			"success_count":   successCount,
			"failure_count":   failureCount,
			"max_concurrency": p.MaxConcurrency,
		},
	}

	if p.CombineOutputs {
		output.ParallelStepOutputs = p.outputs

		// Combine content from all outputs
		if len(p.outputs) > 0 {
			contents := make(map[string]interface{})
			for name, stepOutput := range p.outputs {
				if stepOutput.Content != nil {
					contents[name] = stepOutput.Content
				}
			}
			if len(contents) > 0 {
				output.Content = contents
			}
		}
	}

	return output, nil
}

// executeStep executes a single step
func (p *Parallel) executeStep(ctx context.Context, item interface{}, input *StepInput) (*StepOutput, error) {
	switch v := item.(type) {
	case *Step:
		return v.Execute(ctx, input)
	case ExecutorFunc:
		return v(input)
	case func(*StepInput) (*StepOutput, error):
		return v(input)
	case *Loop:
		return v.Execute(ctx, input)
	case *Parallel:
		return v.Execute(ctx, input)
	case *Condition:
		return v.Execute(ctx, input)
	case *Router:
		return v.Execute(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported step type in parallel: %T", v)
	}
}

// getStepName extracts or generates a name for the step
func (p *Parallel) getStepName(item interface{}, index int) string {
	switch v := item.(type) {
	case *Step:
		if v.Name != "" {
			return v.Name
		}
	case *Loop:
		if v.Name != "" {
			return v.Name
		}
	case *Parallel:
		if v.Name != "" {
			return v.Name
		}
	case *Condition:
		if v.Name != "" {
			return v.Name
		}
	case *Router:
		if v.Name != "" {
			return v.Name
		}
	case ExecutorFunc, func(*StepInput) (*StepOutput, error):
		// For functions, check if they return a StepName in their output
		// For now, generate a name based on index
		return fmt.Sprintf("%s_func_%d", p.Name, index)
	}

	return fmt.Sprintf("%s_step_%d", p.Name, index)
}

// ParallelGroup creates a simple parallel execution group
func ParallelGroup(steps ...interface{}) *Parallel {
	return NewParallel(
		WithParallelSteps(steps...),
		WithCombineOutputs(true),
	)
}

// ParallelRace creates a parallel execution that returns as soon as one completes
func ParallelRace(steps ...interface{}) *Parallel {
	return NewParallel(
		WithParallelSteps(steps...),
		WithFailFast(false),
		WithWaitForAll(false),
	)
}

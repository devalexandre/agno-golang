package v2

import (
	"context"
	"fmt"
	"time"
)

// LoopCondition represents a function that determines whether the loop should continue
type LoopCondition func(iteration int, lastOutput *StepOutput) bool

// Loop represents a construct that executes steps repeatedly
type Loop struct {
	Name        string
	Description string
	Steps       []interface{} // Can contain Steps, other Loops, Parallels, etc.

	// Loop control
	MaxIterations int
	Condition     LoopCondition

	// Configuration
	BreakOnError   bool
	CollectOutputs bool

	// Internal state
	currentIteration int
	outputs          []*StepOutput
}

// NewLoop creates a new Loop instance
func NewLoop(options ...LoopOption) *Loop {
	l := &Loop{
		MaxIterations:  10, // Default max iterations
		CollectOutputs: true,
		outputs:        make([]*StepOutput, 0),
	}

	for _, opt := range options {
		opt(l)
	}

	// Default condition if none provided
	if l.Condition == nil {
		l.Condition = func(iteration int, lastOutput *StepOutput) bool {
			return iteration < l.MaxIterations
		}
	}

	return l
}

// LoopOption is a functional option for configuring a Loop
type LoopOption func(*Loop)

// WithLoopName sets the loop name
func WithLoopName(name string) LoopOption {
	return func(l *Loop) {
		l.Name = name
	}
}

// WithLoopDescription sets the loop description
func WithLoopDescription(desc string) LoopOption {
	return func(l *Loop) {
		l.Description = desc
	}
}

// WithLoopSteps sets the steps to execute in the loop
func WithLoopSteps(steps ...interface{}) LoopOption {
	return func(l *Loop) {
		l.Steps = steps
	}
}

// WithMaxIterations sets the maximum number of iterations
func WithMaxIterations(max int) LoopOption {
	return func(l *Loop) {
		l.MaxIterations = max
	}
}

// WithLoopCondition sets a custom loop condition
func WithLoopCondition(condition LoopCondition) LoopOption {
	return func(l *Loop) {
		l.Condition = condition
	}
}

// WithBreakOnError enables breaking the loop on error
func WithBreakOnError(breakOnError bool) LoopOption {
	return func(l *Loop) {
		l.BreakOnError = breakOnError
	}
}

// WithCollectOutputs enables collecting outputs from all iterations
func WithCollectOutputs(collect bool) LoopOption {
	return func(l *Loop) {
		l.CollectOutputs = collect
	}
}

// Execute runs the loop with the given input
func (l *Loop) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	l.currentIteration = 0
	l.outputs = make([]*StepOutput, 0)

	var lastOutput *StepOutput
	stepInput := &StepInput{
		Message:             input.Message,
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

	startTime := time.Now()

	for l.currentIteration = 0; l.Condition(l.currentIteration, lastOutput); l.currentIteration++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check max iterations
		if l.currentIteration >= l.MaxIterations {
			break
		}

		// Update input with previous iteration output
		if lastOutput != nil {
			stepInput.PreviousStepContent = lastOutput.Content
		}

		// Execute steps for this iteration
		iterationOutput, err := l.executeIteration(ctx, stepInput, l.currentIteration)
		if err != nil {
			if l.BreakOnError {
				return nil, fmt.Errorf("loop '%s' failed at iteration %d: %w", l.Name, l.currentIteration, err)
			}
			// Continue to next iteration even on error
			continue
		}

		// Collect output if configured
		if l.CollectOutputs && iterationOutput != nil {
			l.outputs = append(l.outputs, iterationOutput)
		}

		// Update last output
		lastOutput = iterationOutput

		// Update step input for next iteration
		if iterationOutput != nil {
			iterationKey := fmt.Sprintf("%s_iteration_%d", l.Name, l.currentIteration)
			stepInput.PreviousStepOutputs[iterationKey] = iterationOutput
		}
	}

	endTime := time.Now()

	// Create final output
	output := &StepOutput{
		StepName:     l.Name,
		ExecutorType: "loop",
		Event:        string(LoopExecutionCompletedEvent),
		Metadata: map[string]interface{}{
			"iterations":  l.currentIteration,
			"duration_ms": endTime.Sub(startTime).Milliseconds(),
		},
	}

	if l.CollectOutputs {
		output.LoopStepOutputs = l.outputs
		if len(l.outputs) > 0 {
			// Set content to the last output's content
			output.Content = l.outputs[len(l.outputs)-1].Content
		}
	} else if lastOutput != nil {
		output.Content = lastOutput.Content
	}

	return output, nil
}

// executeIteration executes all steps for a single iteration
func (l *Loop) executeIteration(ctx context.Context, input *StepInput, iteration int) (*StepOutput, error) {
	var lastOutput *StepOutput

	// 🔥 CRIAMOS UM NOVO input para preservar o Message original
	iterInput := &StepInput{
		Message:             input.Message,
		AdditionalData:      input.AdditionalData,
		Images:              input.Images,
		Videos:              input.Videos,
		Audio:               input.Audio,
		PreviousStepContent: input.PreviousStepContent,
		PreviousStepOutputs: map[string]*StepOutput{},
	}

	// 🔁 Copia os outputs anteriores
	for k, v := range input.PreviousStepOutputs {
		iterInput.PreviousStepOutputs[k] = v
	}

	for i, item := range l.Steps {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Atualiza com a saída anterior DENTRO da iteração
		if lastOutput != nil {
			iterInput.PreviousStepContent = lastOutput.Content
		}

		var output *StepOutput
		var err error

		switch v := item.(type) {
		case *Step:
			output, err = v.Execute(ctx, iterInput)
		case ExecutorFunc:
			output, err = v(iterInput)
		case func(*StepInput) (*StepOutput, error):
			output, err = v(iterInput)
		case *Loop:
			output, err = v.Execute(ctx, iterInput)
		case *Parallel:
			output, err = v.Execute(ctx, iterInput)
		case *Condition:
			output, err = v.Execute(ctx, iterInput)
		case *Router:
			output, err = v.Execute(ctx, iterInput)
		default:
			return nil, fmt.Errorf("unsupported step type at index %d in loop '%s': %T", i, l.Name, v)
		}

		if err != nil {
			return nil, err
		}

		if output != nil {
			stepName := fmt.Sprintf("%s_iteration_%d_step_%d", l.Name, iteration, i)
			if output.StepName != "" {
				stepName = fmt.Sprintf("%s_iteration_%d", output.StepName, iteration)
			}
			iterInput.PreviousStepOutputs[stepName] = output
			lastOutput = output
		}
	}

	return lastOutput, nil
}

// Common loop conditions

// WhileTrue creates a condition that continues while a function returns true
func WhileTrue(fn func(int, *StepOutput) bool) LoopCondition {
	return fn
}

// UntilContent creates a condition that continues until specific content is found
func UntilContent(targetContent string) LoopCondition {
	return func(iteration int, lastOutput *StepOutput) bool {
		if lastOutput == nil || lastOutput.Content == nil {
			return true // Continue if no output yet
		}

		contentStr, ok := lastOutput.Content.(string)
		if !ok {
			return true // Continue if content is not a string
		}

		return contentStr != targetContent
	}
}

// ForN creates a condition that runs exactly N times
func ForN(n int) LoopCondition {
	return func(iteration int, lastOutput *StepOutput) bool {
		return iteration < n
	}
}

// WhileError creates a condition that continues while there's an error in metadata
func WhileError() LoopCondition {
	return func(iteration int, lastOutput *StepOutput) bool {
		if lastOutput == nil || lastOutput.Metadata == nil {
			return true
		}

		_, hasError := lastOutput.Metadata["error"]
		return hasError
	}
}

// UntilSuccess creates a condition that continues until success is true in metadata
func UntilSuccess() LoopCondition {
	return func(iteration int, lastOutput *StepOutput) bool {
		if lastOutput == nil || lastOutput.Metadata == nil {
			return true
		}

		success, ok := lastOutput.Metadata["success"].(bool)
		return !ok || !success
	}
}

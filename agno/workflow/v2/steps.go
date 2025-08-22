package v2

import (
	"context"
	"fmt"
)

// Steps represents a collection of steps that can be executed sequentially
type Steps struct {
	Name        string
	Description string
	StepList    []interface{} // Can contain Steps, Loops, Parallels, Conditions, Routers, or ExecutorFuncs

	// Configuration
	ContinueOnError bool // Continue executing remaining steps even if one fails
	CollectOutputs  bool // Collect outputs from all steps

	// Internal state
	outputs map[string]*StepOutput
}

// NewSteps creates a new Steps instance
func NewSteps(options ...StepsOption) *Steps {
	s := &Steps{
		CollectOutputs: true,
		outputs:        make(map[string]*StepOutput),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

// StepsOption is a functional option for configuring Steps
type StepsOption func(*Steps)

// WithStepsName sets the steps collection name
func WithStepsName(name string) StepsOption {
	return func(s *Steps) {
		s.Name = name
	}
}

// WithStepsDescription sets the steps collection description
func WithStepsDescription(desc string) StepsOption {
	return func(s *Steps) {
		s.Description = desc
	}
}

// WithStepsList sets the list of steps
func WithStepsList(steps ...interface{}) StepsOption {
	return func(s *Steps) {
		s.StepList = steps
	}
}

// WithStepsContinueOnError enables continuing execution on error
func WithStepsContinueOnError(continueOnError bool) StepsOption {
	return func(s *Steps) {
		s.ContinueOnError = continueOnError
	}
}

// WithCollectStepsOutputs enables collecting outputs from all steps
func WithCollectStepsOutputs(collect bool) StepsOption {
	return func(s *Steps) {
		s.CollectOutputs = collect
	}
}

// Add adds a new step to the collection
func (s *Steps) Add(step interface{}) {
	s.StepList = append(s.StepList, step)
}

// Execute runs all steps sequentially with the given input
func (s *Steps) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	if len(s.StepList) == 0 {
		return &StepOutput{
			StepName:     s.Name,
			ExecutorType: "steps",
			Event:        string(StepsExecutionCompletedEvent),
			Metadata: map[string]interface{}{
				"message": "no steps to execute",
			},
		}, nil
	}

	// Reset outputs
	s.outputs = make(map[string]*StepOutput)

	var lastOutput *StepOutput
	var errors []error
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

	successCount := 0
	failureCount := 0

	for i, item := range s.StepList {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Update input with previous step output
		if lastOutput != nil {
			stepInput.PreviousStepContent = lastOutput.Content
		}

		var output *StepOutput
		var err error

		// Execute based on step type
		switch v := item.(type) {
		case *Step:
			output, err = v.Execute(ctx, stepInput)
		case ExecutorFunc:
			output, err = v(stepInput)
		case *Steps:
			output, err = v.Execute(ctx, stepInput)
		case *Loop:
			output, err = v.Execute(ctx, stepInput)
		case *Parallel:
			output, err = v.Execute(ctx, stepInput)
		case *Condition:
			output, err = v.Execute(ctx, stepInput)
		case *Router:
			output, err = v.Execute(ctx, stepInput)
		default:
			err = fmt.Errorf("unsupported step type at index %d: %T", i, v)
		}

		if err != nil {
			failureCount++
			errors = append(errors, fmt.Errorf("step %d failed: %w", i, err))

			if !s.ContinueOnError {
				return nil, fmt.Errorf("steps '%s' failed at step %d: %w", s.Name, i, err)
			}
			// Continue to next step if ContinueOnError is true
			continue
		}

		successCount++

		if output != nil {
			// Determine step name
			stepName := fmt.Sprintf("%s_step_%d", s.Name, i)
			if output.StepName != "" {
				stepName = output.StepName
			}

			// Store output
			if s.CollectOutputs {
				s.outputs[stepName] = output
			}

			// Update step input for next iteration
			stepInput.PreviousStepOutputs[stepName] = output
			lastOutput = output
		}
	}

	// Create final output
	finalOutput := &StepOutput{
		StepName:     s.Name,
		ExecutorType: "steps",
		Event:        string(StepsExecutionCompletedEvent),
		Metadata: map[string]interface{}{
			"total_steps":   len(s.StepList),
			"success_count": successCount,
			"failure_count": failureCount,
		},
	}

	// Add errors to metadata if any
	if len(errors) > 0 {
		errorMessages := make([]string, len(errors))
		for i, err := range errors {
			errorMessages[i] = err.Error()
		}
		finalOutput.Metadata["errors"] = errorMessages
	}

	// Set content and outputs
	if s.CollectOutputs && len(s.outputs) > 0 {
		// Create a map of step outputs
		outputMap := make(map[string]interface{})
		for name, output := range s.outputs {
			if output.Content != nil {
				outputMap[name] = output.Content
			}
		}

		if len(outputMap) > 0 {
			finalOutput.Content = outputMap
		}

		// Also store the full outputs
		finalOutput.ParallelStepOutputs = s.outputs
	} else if lastOutput != nil {
		// If not collecting outputs, just use the last output's content
		finalOutput.Content = lastOutput.Content
	}

	// Return error if all steps failed and ContinueOnError is true
	if failureCount == len(s.StepList) && len(errors) > 0 {
		return finalOutput, fmt.Errorf("all steps failed: %v", errors)
	}

	return finalOutput, nil
}

// GetOutput returns the output of a specific step by name
func (s *Steps) GetOutput(stepName string) *StepOutput {
	return s.outputs[stepName]
}

// GetAllOutputs returns all collected outputs
func (s *Steps) GetAllOutputs() map[string]*StepOutput {
	return s.outputs
}

// Clear clears all collected outputs
func (s *Steps) Clear() {
	s.outputs = make(map[string]*StepOutput)
	s.StepList = make([]interface{}, 0)
}

// Len returns the number of steps
func (s *Steps) Len() int {
	return len(s.StepList)
}

// Sequential creates a sequential execution of steps
func Sequential(steps ...interface{}) *Steps {
	return NewSteps(
		WithStepsList(steps...),
		WithCollectStepsOutputs(true),
	)
}

// Pipeline creates a pipeline of steps where output flows from one to the next
func Pipeline(name string, steps ...interface{}) *Steps {
	return NewSteps(
		WithStepsName(name),
		WithStepsList(steps...),
		WithCollectStepsOutputs(false),  // Only keep last output
		WithStepsContinueOnError(false), // Stop on first error
	)
}

// TryAll creates a steps collection that tries all steps regardless of failures
func TryAll(steps ...interface{}) *Steps {
	return NewSteps(
		WithStepsList(steps...),
		WithStepsContinueOnError(true),
		WithCollectStepsOutputs(true),
	)
}

// Chain creates a simple chain of steps
func Chain(steps ...interface{}) []interface{} {
	return steps
}

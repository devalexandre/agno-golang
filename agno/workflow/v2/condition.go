package v2

import (
	"context"
	"fmt"
)

// ConditionFunc represents a function that evaluates a condition
type ConditionFunc func(*StepInput) bool

// Condition represents a conditional execution construct
type Condition struct {
	Name        string
	Description string

	// The condition to evaluate
	If ConditionFunc

	// Steps to execute if condition is true
	Then []interface{}

	// Steps to execute if condition is false (optional)
	Else []interface{}

	// Configuration
	EvaluateAsync bool // Evaluate condition asynchronously

	// Internal state
	conditionResult bool
	executedBranch  string
}

// NewCondition creates a new Condition instance
func NewCondition(options ...ConditionOption) *Condition {
	c := &Condition{}

	for _, opt := range options {
		opt(c)
	}

	return c
}

// ConditionOption is a functional option for configuring a Condition
type ConditionOption func(*Condition)

// WithConditionName sets the condition name
func WithConditionName(name string) ConditionOption {
	return func(c *Condition) {
		c.Name = name
	}
}

// WithConditionDescription sets the condition description
func WithConditionDescription(desc string) ConditionOption {
	return func(c *Condition) {
		c.Description = desc
	}
}

// WithIf sets the condition function
func WithIf(condition ConditionFunc) ConditionOption {
	return func(c *Condition) {
		c.If = condition
	}
}

// WithThen sets the steps to execute if condition is true
func WithThen(steps ...interface{}) ConditionOption {
	return func(c *Condition) {
		c.Then = steps
	}
}

// WithElse sets the steps to execute if condition is false
func WithElse(steps ...interface{}) ConditionOption {
	return func(c *Condition) {
		c.Else = steps
	}
}

// WithEvaluateAsync enables asynchronous condition evaluation
func WithEvaluateAsync(async bool) ConditionOption {
	return func(c *Condition) {
		c.EvaluateAsync = async
	}
}

// Execute evaluates the condition and executes the appropriate branch
func (c *Condition) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	if c.If == nil {
		return nil, fmt.Errorf("condition '%s' has no evaluation function", c.Name)
	}

	// Evaluate the condition
	c.conditionResult = c.If(input)

	// Determine which branch to execute
	var stepsToExecute []interface{}
	if c.conditionResult {
		c.executedBranch = "then"
		stepsToExecute = c.Then
	} else {
		c.executedBranch = "else"
		stepsToExecute = c.Else
	}

	// If no steps to execute, return early
	if len(stepsToExecute) == 0 {
		return &StepOutput{
			StepName:     c.Name,
			ExecutorType: "condition",
			Event:        string(ConditionExecutionCompletedEvent),
			Metadata: map[string]interface{}{
				"condition_result": c.conditionResult,
				"executed_branch":  c.executedBranch,
				"message":          fmt.Sprintf("no steps defined for %s branch", c.executedBranch),
			},
		}, nil
	}

	// Execute the selected branch
	var lastOutput *StepOutput
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

	for i, item := range stepsToExecute {
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

		switch v := item.(type) {
		case *Step:
			output, err = v.Execute(ctx, stepInput)
		case ExecutorFunc:
			output, err = v(stepInput)
		case func(*StepInput) (*StepOutput, error):
			output, err = v(stepInput)
		case *Loop:
			output, err = v.Execute(ctx, stepInput)
		case *Parallel:
			output, err = v.Execute(ctx, stepInput)
		case *Condition:
			output, err = v.Execute(ctx, stepInput)
		case *Router:
			output, err = v.Execute(ctx, stepInput)
		default:
			return nil, fmt.Errorf("unsupported step type at index %d in condition '%s': %T", i, c.Name, v)
		}

		if err != nil {
			return nil, fmt.Errorf("condition '%s' branch '%s' step %d failed: %w", c.Name, c.executedBranch, i, err)
		}

		if output != nil {
			stepName := fmt.Sprintf("%s_%s_step_%d", c.Name, c.executedBranch, i)
			if output.StepName != "" {
				stepName = fmt.Sprintf("%s_%s", output.StepName, c.executedBranch)
			}

			stepInput.PreviousStepOutputs[stepName] = output
			lastOutput = output
		}
	}

	// Create final output
	finalOutput := &StepOutput{
		StepName:     c.Name,
		ExecutorType: "condition",
		Event:        string(ConditionExecutionCompletedEvent),
		Metadata: map[string]interface{}{
			"condition_result": c.conditionResult,
			"executed_branch":  c.executedBranch,
			"steps_executed":   len(stepsToExecute),
		},
	}

	if lastOutput != nil {
		finalOutput.Content = lastOutput.Content
	}

	return finalOutput, nil
}

// Common condition functions

// IfTrue creates a condition that evaluates to true if the function returns true
func IfTrue(fn func(*StepInput) bool) ConditionFunc {
	return fn
}

// IfContentEquals creates a condition that checks if content equals a value
func IfContentEquals(target string) ConditionFunc {
	return func(input *StepInput) bool {
		if input.PreviousStepContent == nil {
			return false
		}

		contentStr, ok := input.PreviousStepContent.(string)
		if !ok {
			return false
		}

		return contentStr == target
	}
}

// IfContentContains creates a condition that checks if content contains a substring
func IfContentContains(substring string) ConditionFunc {
	return func(input *StepInput) bool {
		if input.PreviousStepContent == nil {
			return false
		}

		contentStr, ok := input.PreviousStepContent.(string)
		if !ok {
			return false
		}

		return contains(contentStr, substring)
	}
}

// IfHasOutput creates a condition that checks if a specific step has output
func IfHasOutput(stepName string) ConditionFunc {
	return func(input *StepInput) bool {
		return input.GetStepOutput(stepName) != nil
	}
}

// IfMetadataExists creates a condition that checks if metadata key exists
func IfMetadataExists(key string) ConditionFunc {
	return func(input *StepInput) bool {
		if input.AdditionalData == nil {
			return false
		}

		_, exists := input.AdditionalData[key]
		return exists
	}
}

// IfMetadataEquals creates a condition that checks if metadata value equals
func IfMetadataEquals(key string, value interface{}) ConditionFunc {
	return func(input *StepInput) bool {
		if input.AdditionalData == nil {
			return false
		}

		val, exists := input.AdditionalData[key]
		if !exists {
			return false
		}

		return val == value
	}
}

// IfAnd combines multiple conditions with AND logic
func IfAnd(conditions ...ConditionFunc) ConditionFunc {
	return func(input *StepInput) bool {
		for _, condition := range conditions {
			if !condition(input) {
				return false
			}
		}
		return true
	}
}

// IfOr combines multiple conditions with OR logic
func IfOr(conditions ...ConditionFunc) ConditionFunc {
	return func(input *StepInput) bool {
		for _, condition := range conditions {
			if condition(input) {
				return true
			}
		}
		return false
	}
}

// IfNot negates a condition
func IfNot(condition ConditionFunc) ConditionFunc {
	return func(input *StepInput) bool {
		return !condition(input)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

// SimpleIf creates a simple if-then-else condition
func SimpleIf(condition ConditionFunc, thenStep interface{}, elseStep interface{}) *Condition {
	opts := []ConditionOption{
		WithIf(condition),
	}

	if thenStep != nil {
		opts = append(opts, WithThen(thenStep))
	}

	if elseStep != nil {
		opts = append(opts, WithElse(elseStep))
	}

	return NewCondition(opts...)
}

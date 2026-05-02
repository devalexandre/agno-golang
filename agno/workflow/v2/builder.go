package v2

import (
	"context"
)

// FlowBuilder provides a fluent API for constructing complex workflows.
type FlowBuilder struct {
	name        string
	description string
	steps       []interface{}
	storage     Storage
	durable     bool
	debug       bool
}

// NewFlow starts a new fluent workflow definition.
func NewFlow(name string) *FlowBuilder {
	return &FlowBuilder{
		name: name,
	}
}

// Description sets the workflow description.
func (b *FlowBuilder) Description(desc string) *FlowBuilder {
	b.description = desc
	return b
}

// Storage sets the workflow storage.
func (b *FlowBuilder) Storage(s Storage) *FlowBuilder {
	b.storage = s
	return b
}

// Durable enables or disables workflow durability.
func (b *FlowBuilder) Durable(durable bool) *FlowBuilder {
	b.durable = durable
	return b
}

// Debug enables or disables debug mode.
func (b *FlowBuilder) Debug(debug bool) *FlowBuilder {
	b.debug = debug
	return b
}

// Step adds a simple step to the workflow.
func (b *FlowBuilder) Step(name string, executor interface{}, options ...StepOption) *FlowBuilder {
	var s *Step
	var err error

	opts := append([]StepOption{WithName(name)}, options...)

	switch e := executor.(type) {
	case Agent:
		opts = append(opts, WithAgent(e))
	case Team:
		opts = append(opts, WithTeam(e))
	case ExecutorFunc:
		opts = append(opts, WithExecutor(e))
	case func(*StepInput) (*StepOutput, error):
		opts = append(opts, WithExecutor(e))
	}

	s, err = NewStep(opts...)
	if err == nil {
		b.steps = append(b.steps, s)
	}
	return b
}

// If adds a conditional branching to the workflow.
func (b *FlowBuilder) If(condition ConditionFunc, thenSteps ...interface{}) *FlowBuilder {
	c := NewCondition(
		WithIf(condition),
		WithThen(thenSteps...),
	)
	b.steps = append(b.steps, c)
	return b
}

// IfElse adds a conditional branching with an else clause.
func (b *FlowBuilder) IfElse(condition ConditionFunc, thenSteps []interface{}, elseSteps []interface{}) *FlowBuilder {
	c := NewCondition(
		WithIf(condition),
		WithThen(thenSteps...),
		WithElse(elseSteps...),
	)
	b.steps = append(b.steps, c)
	return b
}

// Loop adds a loop to the workflow.
func (b *FlowBuilder) Loop(condition LoopCondition, steps ...interface{}) *FlowBuilder {
	l := NewLoop(
		WithLoopCondition(condition),
		WithLoopSteps(steps...),
	)
	b.steps = append(b.steps, l)
	return b
}

// Parallel adds a parallel execution block.
func (b *FlowBuilder) Parallel(name string, steps ...interface{}) *FlowBuilder {
	p := NewParallel(
		WithParallelName(name),
		WithParallelSteps(steps...),
	)
	b.steps = append(b.steps, p)
	return b
}

// Build creates the final Workflow instance.
func (b *FlowBuilder) Build() *Workflow {
	opts := []WorkflowOption{
		WithWorkflowName(b.name),
		WithWorkflowDescription(b.description),
		WithWorkflowSteps(Sequential(b.steps...)),
		WithDurable(b.durable),
		WithDebugMode(b.debug),
	}

	if b.storage != nil {
		opts = append(opts, WithStorage(b.storage))
	}

	return NewWorkflow(opts...)
}

// Run creates and executes the workflow in one go.
func (b *FlowBuilder) Run(ctx context.Context, input interface{}) (*WorkflowRunResponse, error) {
	return b.Build().Run(ctx, input)
}

package flow

import (
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// FlowBuilder is a fluent API for constructing workflows.
type FlowBuilder struct {
	name        string
	description string
	steps       []interface{}
	debug       bool
}

// New creates a new FlowBuilder with the given name.
func New(name string) *FlowBuilder {
	return &FlowBuilder{
		name: name,
	}
}

// Description sets the description of the workflow.
func (b *FlowBuilder) Description(desc string) *FlowBuilder {
	b.description = desc
	return b
}

// Debug enables or disables debug mode.
func (b *FlowBuilder) Debug(debug bool) *FlowBuilder {
	b.debug = debug
	return b
}

// Step adds a new step to the workflow.
// executor can be an Agent, Team, or ExecutorFunc.
func (b *FlowBuilder) Step(name string, executor any, options ...v2.StepOption) *FlowBuilder {
	opts := append([]v2.StepOption{v2.WithName(name)}, options...)

	switch e := executor.(type) {
	case v2.Agent:
		opts = append(opts, v2.WithAgent(e))
	case v2.Team:
		opts = append(opts, v2.WithTeam(e))
	case v2.ExecutorFunc:
		opts = append(opts, v2.WithExecutor(e))
	case func(*v2.StepInput) (*v2.StepOutput, error):
		opts = append(opts, v2.WithExecutor(v2.ExecutorFunc(e)))
	}

	step, _ := v2.NewStep(opts...)
	b.steps = append(b.steps, step)
	return b
}

// IfSuccess returns a condition function that checks if the previous step was successful.
func IfSuccess() v2.ConditionFunc {
	return func(input *v2.StepInput) bool {
		lastOutput := input.GetLastStepContent()
		return lastOutput != nil
	}
}

// If adds a conditional step to the workflow.
func (b *FlowBuilder) If(condition v2.ConditionFunc, thenSteps ...any) *ConditionBuilder {
	cond := v2.NewCondition(
		v2.WithIf(condition),
		v2.WithThen(b.convertToInterfaces(thenSteps)...),
	)
	b.steps = append(b.steps, cond)
	return &ConditionBuilder{
		builder:   b,
		condition: cond,
	}
}

// Loop adds a loop step to the workflow.
func (b *FlowBuilder) Loop(condition v2.LoopCondition, steps ...any) *FlowBuilder {
	loop := v2.NewLoop(
		v2.WithLoopCondition(condition),
		v2.WithLoopSteps(b.convertToInterfaces(steps)...),
	)
	b.steps = append(b.steps, loop)
	return b
}

// Parallel adds a parallel step to the workflow.
func (b *FlowBuilder) Parallel(steps ...any) *FlowBuilder {
	parallel := v2.NewParallel(
		v2.WithParallelSteps(b.convertToInterfaces(steps)...),
	)
	b.steps = append(b.steps, parallel)
	return b
}

// Build constructs and returns the final Workflow.
func (b *FlowBuilder) Build() *v2.Workflow {
	opts := []v2.WorkflowOption{
		v2.WithWorkflowName(b.name),
		v2.WithWorkflowDescription(b.description),
		v2.WithWorkflowSteps(b.steps),
		v2.WithDebugMode(b.debug),
	}
	return v2.NewWorkflow(opts...)
}

func (b *FlowBuilder) convertToInterfaces(steps []any) []interface{} {
	interfaces := make([]interface{}, len(steps))
	for i, s := range steps {
		interfaces[i] = s
	}
	return interfaces
}

// ConditionBuilder is a helper for building conditional steps with Else.
type ConditionBuilder struct {
	builder   *FlowBuilder
	condition *v2.Condition
}

// Else adds an else branch to the current condition.
func (cb *ConditionBuilder) Else(elseSteps ...any) *FlowBuilder {
	v2.WithElse(cb.builder.convertToInterfaces(elseSteps)...)(cb.condition)
	return cb.builder
}

// End returns the FlowBuilder from the ConditionBuilder.
func (cb *ConditionBuilder) End() *FlowBuilder {
	return cb.builder
}

// Step continues adding steps to the FlowBuilder.
func (cb *ConditionBuilder) Step(name string, executor any, options ...v2.StepOption) *FlowBuilder {
	return cb.builder.Step(name, executor, options...)
}

package v2

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

// StepExecutor represents any type that can execute a step
type StepExecutor interface {
	Execute(ctx context.Context, input *StepInput) (*StepOutput, error)
}

// Step represents a single unit of work in a workflow pipeline
type Step struct {
	Name        string
	StepID      string
	Description string

	// Executor options - only one should be provided
	Agent    Agent
	Team     Team
	Executor ExecutorFunc

	// Step configuration
	MaxRetries            int
	TimeoutSeconds        int
	SkipOnFailure         bool
	StrictInputValidation bool

	// Internal state
	activeExecutor StepExecutor
	executorType   string
	retryCount     int
}

// Agent interface (should be imported from agent package)
type Agent interface {
	Run(ctx context.Context, message string, options ...interface{}) (interface{}, error)
	GetName() string
}

// Team interface (should be imported from team package)
type Team interface {
	Run(ctx context.Context, message string, options ...interface{}) (interface{}, error)
	GetName() string
}

// NewStep creates a new Step with the provided configuration
func NewStep(options ...StepOption) (*Step, error) {
	s := &Step{
		MaxRetries:            3,
		StrictInputValidation: false,
	}

	for _, opt := range options {
		opt(s)
	}

	// Auto-detect name for function executors if not provided
	if s.Name == "" && s.Executor != nil {
		s.Name = getFunctionName(s.Executor)
	}

	// Validate executor configuration
	if err := s.validateExecutorConfig(); err != nil {
		return nil, err
	}

	// Set the active executor
	s.setActiveExecutor()

	return s, nil
}

// StepOption is a functional option for configuring a Step
type StepOption func(*Step)

// WithName sets the step name
func WithName(name string) StepOption {
	return func(s *Step) {
		s.Name = name
	}
}

// WithAgent sets an agent as the executor
func WithAgent(agent Agent) StepOption {
	return func(s *Step) {
		s.Agent = agent
	}
}

// WithTeam sets a team as the executor
func WithTeam(team Team) StepOption {
	return func(s *Step) {
		s.Team = team
	}
}

// WithExecutor sets a function as the executor
func WithExecutor(executor ExecutorFunc) StepOption {
	return func(s *Step) {
		s.Executor = executor
	}
}

// WithDescription sets the step description
func WithDescription(desc string) StepOption {
	return func(s *Step) {
		s.Description = desc
	}
}

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(retries int) StepOption {
	return func(s *Step) {
		s.MaxRetries = retries
	}
}

// WithTimeout sets the timeout in seconds
func WithTimeout(seconds int) StepOption {
	return func(s *Step) {
		s.TimeoutSeconds = seconds
	}
}

// WithSkipOnFailure enables skipping on failure
func WithSkipOnFailure(skip bool) StepOption {
	return func(s *Step) {
		s.SkipOnFailure = skip
	}
}

// WithStrictValidation enables strict input validation
func WithStrictValidation(strict bool) StepOption {
	return func(s *Step) {
		s.StrictInputValidation = strict
	}
}

// GetExecutorName returns the name of the current executor
func (s *Step) GetExecutorName() string {
	switch s.executorType {
	case "agent":
		if s.Agent != nil {
			return s.Agent.GetName()
		}
		return "unnamed_agent"
	case "team":
		if s.Team != nil {
			return s.Team.GetName()
		}
		return "unnamed_team"
	case "function":
		if s.Executor != nil {
			return getFunctionName(s.Executor)
		}
		return "anonymous_function"
	default:
		return fmt.Sprintf("%s_executor", s.executorType)
	}
}

// GetExecutorType returns the type of the current executor
func (s *Step) GetExecutorType() string {
	return s.executorType
}

// validateExecutorConfig validates that only one executor type is provided
func (s *Step) validateExecutorConfig() error {
	executorCount := 0
	var providedExecutors []string

	if s.Agent != nil {
		executorCount++
		providedExecutors = append(providedExecutors, "agent")
	}
	if s.Team != nil {
		executorCount++
		providedExecutors = append(providedExecutors, "team")
	}
	if s.Executor != nil {
		executorCount++
		providedExecutors = append(providedExecutors, "executor")
	}

	if executorCount == 0 {
		return fmt.Errorf("step '%s' must have one executor: agent, team, or executor", s.Name)
	}

	if executorCount > 1 {
		return fmt.Errorf(
			"step '%s' can only have one executor type. Provided: %v. Please use only one of: agent, team, or executor",
			s.Name, providedExecutors,
		)
	}

	return nil
}

// setActiveExecutor sets the active executor based on what was provided
func (s *Step) setActiveExecutor() {
	if s.Agent != nil {
		s.activeExecutor = &agentExecutor{agent: s.Agent}
		s.executorType = "agent"
	} else if s.Team != nil {
		s.activeExecutor = &teamExecutor{team: s.Team}
		s.executorType = "team"
	} else if s.Executor != nil {
		s.activeExecutor = &functionExecutor{fn: s.Executor}
		s.executorType = "function"
	}
}

// Execute runs the step with the given input
func (s *Step) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	// Apply timeout if configured
	if s.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Execute with retry logic
	var lastErr error
	for attempt := 0; attempt <= s.MaxRetries; attempt++ {
		if attempt > 0 {
			// Log retry attempt
			fmt.Printf("Retrying step '%s' (attempt %d/%d)\n", s.Name, attempt, s.MaxRetries)
		}

		output, err := s.executeOnce(ctx, input)
		if err == nil {
			return output, nil
		}

		lastErr = err
		s.retryCount = attempt

		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			break
		}

		// Wait before retry (exponential backoff)
		if attempt < s.MaxRetries {
			backoff := time.Duration(attempt+1) * time.Second
			select {
			case <-time.After(backoff):
				// Continue to next retry
			case <-ctx.Done():
				break
			}
		}
	}

	// Handle failure based on configuration
	if s.SkipOnFailure {
		return &StepOutput{
			StepName:     s.Name,
			ExecutorName: s.GetExecutorName(),
			ExecutorType: s.GetExecutorType(),
			Event:        "StepSkipped",
			Metadata: map[string]interface{}{
				"error":  lastErr.Error(),
				"reason": "skip_on_failure",
			},
		}, nil
	}

	return nil, fmt.Errorf("step '%s' failed after %d retries: %w", s.Name, s.retryCount, lastErr)
}

// executeOnce executes the step once without retry logic
func (s *Step) executeOnce(ctx context.Context, input *StepInput) (*StepOutput, error) {
	// Validate input if strict validation is enabled
	if s.StrictInputValidation {
		if err := s.validateInput(input); err != nil {
			return nil, fmt.Errorf("input validation failed: %w", err)
		}
	}

	// Execute using the active executor
	output, err := s.activeExecutor.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	// Enrich output with step metadata
	if output.StepName == "" {
		output.StepName = s.Name
	}
	if output.ExecutorName == "" {
		output.ExecutorName = s.GetExecutorName()
	}
	if output.ExecutorType == "" {
		output.ExecutorType = s.GetExecutorType()
	}

	return output, nil
}

// validateInput validates the step input
func (s *Step) validateInput(input *StepInput) error {
	if input == nil {
		return fmt.Errorf("input is nil")
	}

	// Add custom validation logic here if needed
	// For example, check required fields based on executor type

	return nil
}

// Executor adapters

// agentExecutor adapts an Agent to the StepExecutor interface
type agentExecutor struct {
	agent Agent
}

func (e *agentExecutor) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	message := input.GetMessageAsString()
	if message == "" && input.PreviousStepContent != nil {
		message = fmt.Sprintf("%v", input.PreviousStepContent)
	}

	result, err := e.agent.Run(ctx, message)
	if err != nil {
		return nil, err
	}

	return &StepOutput{
		Content: result,
	}, nil
}

// teamExecutor adapts a Team to the StepExecutor interface
type teamExecutor struct {
	team Team
}

func (e *teamExecutor) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	message := input.GetMessageAsString()
	if message == "" && input.PreviousStepContent != nil {
		message = fmt.Sprintf("%v", input.PreviousStepContent)
	}

	result, err := e.team.Run(ctx, message)
	if err != nil {
		return nil, err
	}

	return &StepOutput{
		Content: result,
	}, nil
}

// functionExecutor adapts an ExecutorFunc to the StepExecutor interface
type functionExecutor struct {
	fn ExecutorFunc
}

func (e *functionExecutor) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	// Check if context is cancelled before executing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return e.fn(input)
}

// Helper function to get function name using reflection
func getFunctionName(fn interface{}) string {
	if fn == nil {
		return "nil_function"
	}

	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		return "not_a_function"
	}

	fnType := fnValue.Type()
	if fnType.Name() != "" {
		return fnType.Name()
	}

	// Try to get the name from the pointer
	fnPtr := fnValue.Pointer()
	if fnPtr != 0 {
		return fmt.Sprintf("func_%x", fnPtr)
	}

	return "anonymous_function"
}

// AsyncStep represents a step that can execute asynchronously
type AsyncStep struct {
	*Step
	AsyncExecutor AsyncExecutorFunc
}

// ExecuteAsync runs the step asynchronously
func (s *AsyncStep) ExecuteAsync(ctx context.Context, input *StepInput) (<-chan *StepOutput, error) {
	if s.AsyncExecutor != nil {
		return s.AsyncExecutor(input)
	}

	// Fall back to synchronous execution in a goroutine
	outputChan := make(chan *StepOutput, 1)
	go func() {
		defer close(outputChan)
		output, err := s.Execute(ctx, input)
		if err != nil {
			output = &StepOutput{
				StepName: s.Name,
				Event:    "StepError",
				Metadata: map[string]interface{}{
					"error": err.Error(),
				},
			}
		}
		outputChan <- output
	}()

	return outputChan, nil
}

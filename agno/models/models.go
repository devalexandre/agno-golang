package models

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools"
)

type Role string

const (
	TypeSystemRole    = "system"
	TypeUserRole      = "user"
	TypeAssistantRole = "assistant"
	TypeToolRole      = "tool"
)

type contextKey string

const DebugKey contextKey = "debug"
const ShowToolsCallKey contextKey = "showToolsCall"

type Message struct {
	Role       Role             `json:"role"`
	Content    string           `json:"content"`
	ToolCallID *string          `json:"tool_call_id,omitempty"`
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`
	Thinking   string           `json:"thinking,omitempty"`
}

type MessageResponse struct {
	Model            string           `json:"model"`
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	Thinking         string           `json:"thinking,omitempty"`
	ToolCalls        []tools.ToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
}

func (r Role) IsValid() bool {
	switch r {
	case TypeUserRole, TypeAssistantRole:
		return true
	default:
		return false
	}
}

// NextAction defines the possible next actions in the reasoning process
type NextAction string

const (
	// Continue indicates the reasoning should continue to the next step
	Continue NextAction = "continue"
	// Validate indicates the current result should be validated
	Validate NextAction = "validate"
	// FinalAnswer indicates the final answer has been reached
	FinalAnswer NextAction = "final_answer"
	// Reset indicates the reasoning process should be reset
	Reset NextAction = "reset"
)

// IsValid checks if the NextAction has a valid value
func (a NextAction) IsValid() bool {
	switch a {
	case Continue, Validate, FinalAnswer, Reset:
		return true
	default:
		return false
	}
}

// String returns the string representation of NextAction
func (a NextAction) String() string {
	return string(a)
}

// ParseNextAction parses a string into a NextAction
func ParseNextAction(s string) (NextAction, error) {
	action := NextAction(s)
	if !action.IsValid() {
		return "", fmt.Errorf("invalid NextAction: %s", s)
	}
	return action, nil
}

// ReasoningStep represents a single step in the reasoning process
type ReasoningStep struct {
	// Title is a concise title summarizing the step's purpose
	Title string `json:"title,omitempty"`
	// Action is the action derived from this step (first person perspective like "I will...")
	Action string `json:"action,omitempty"`
	// Result is the result of executing the action (first person perspective like "I did...")
	Result string `json:"result,omitempty"`
	// Reasoning contains the thought process and considerations behind this step
	Reasoning string `json:"reasoning,omitempty"`
	// NextAction indicates what to do next in the reasoning process
	NextAction NextAction `json:"next_action,omitempty"`
	// Confidence is a score between 0.0 and 1.0 indicating confidence in this step
	Confidence float64 `json:"confidence,omitempty"`
}

// Validate checks if the ReasoningStep is valid
func (r *ReasoningStep) Validate() error {
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1, got %f", r.Confidence)
	}
	if !r.NextAction.IsValid() && r.NextAction != "" {
		return fmt.Errorf("invalid NextAction: %s", r.NextAction)
	}
	return nil
}

// ReasoningSteps is a container for a list of reasoning steps
type ReasoningSteps struct {
	// Steps contains the list of reasoning steps
	Steps []ReasoningStep `json:"reasoning_steps"`
}

// Validate checks if all ReasoningSteps are valid
func (r *ReasoningSteps) Validate() error {
	for i, step := range r.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("invalid step at index %d: %w", i, err)
		}
	}
	return nil
}

// AgnoModelInterface represents the interface for integration with language models.
type ReasoningAgentInterface interface {
	Reason(prompt string) ([]ReasoningStep, error)
}

type AgnoModelInterface interface {
	Invoke(ctx context.Context, messages []Message, options ...Option) (*MessageResponse, error)
	AInvoke(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error)
	InvokeStream(ctx context.Context, messages []Message, options ...Option) error
	AInvokeStream(ctx context.Context, messages []Message, options ...Option) (<-chan *MessageResponse, <-chan error)
	GetID() string
}

// AgentInterface define os métodos essenciais para agentes
// Note: RunOption is defined in agno/agent/options.go
type AgentInterface interface {
	Run(input interface{}, opts ...interface{}) (RunResponse, error)
	Reason(prompt string) ([]ReasoningStep, error)
	RunStream(prompt string, fn func([]byte) error) error
}

type RunResponse struct {
	TextContent        string                   `json:"text_content,omitempty"`
	ContentType        string                   `json:"content_type,omitempty"`
	Event              string                   `json:"event,omitempty"`
	Messages           []Message                `json:"messages,omitempty"`
	Metrics            map[string]interface{}   `json:"metrics,omitempty"`
	Model              string                   `json:"model,omitempty"`
	RunID              string                   `json:"run_id,omitempty"`
	AgentID            string                   `json:"agent_id,omitempty"`
	SessionID          string                   `json:"session_id,omitempty"`
	WorkflowID         string                   `json:"workflow_id,omitempty"`
	Tools              []map[string]interface{} `json:"tools,omitempty"`
	FormattedToolCalls []string                 `json:"formatted_tool_calls,omitempty"`
	CreatedAt          int64                    `json:"created_at,omitempty"`
	ParsedOutput       interface{}              `json:"parsed_output,omitempty"` // Deprecated: Use Output instead
	Output             interface{}              `json:"output,omitempty"`        // Structured output when using OutputSchema (already type-asserted)
	// TODO: implement images, videos, audio, response_audio, citations, extra_data
}

// TypedRunResponse is a generic wrapper for RunResponse with typed Output
type TypedRunResponse[T any] struct {
	RunResponse
	Output T `json:"output,omitempty"` // Typed output field
}

// GetOutput returns the output with the correct type (generic helper)
// Usage: movieScript := run.GetOutput(MovieScript{})
func (r *RunResponse) GetOutput(target interface{}) interface{} {
	if r.Output == nil {
		return target // Return zero value of type
	}
	return r.Output
}

// Type aliases for backwards compatibility
// These allow other packages to reference agent types via models package
// Import agent package to get access to these types
// Note: These are defined in agno/agent/options.go
// type RunOption = agent.RunOption
// type Image = agent.Image
// type Audio = agent.Audio
// type Video = agent.Video
// type File = agent.File

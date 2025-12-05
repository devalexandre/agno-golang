package agent

import (
	"context"
	"fmt"
	"strings"
)

type ChainToolErrorHandler interface {
	Handle(ctx context.Context, toolName string, err error, previousResults map[string]interface{}) (shouldContinue bool, recovery interface{})
}

type RollbackStrategy string

const (
	RollbackNone       RollbackStrategy = "none"
	RollbackToStart    RollbackStrategy = "to_start"
	RollbackToPrevious RollbackStrategy = "to_previous"
	RollbackSkip       RollbackStrategy = "skip"
)

type ChainToolErrorConfig struct {
	Strategy       RollbackStrategy
	MaxRetries     int
	CustomHandlers map[string]ChainToolErrorHandler
	OnError        func(toolName string, err error) error
}

type DefaultErrorHandler struct {
	strategy   RollbackStrategy
	maxRetries int
}

func NewDefaultErrorHandler(strategy RollbackStrategy, maxRetries int) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		strategy:   strategy,
		maxRetries: maxRetries,
	}
}

func (h *DefaultErrorHandler) Handle(ctx context.Context, toolName string, err error, previousResults map[string]interface{}) (shouldContinue bool, recovery interface{}) {
	switch h.strategy {
	case RollbackNone:
		return false, nil

	case RollbackToStart:
		return true, previousResults

	case RollbackToPrevious:
		if len(previousResults) > 0 {
			return true, previousResults
		}
		return false, nil

	case RollbackSkip:
		return true, nil

	default:
		return false, nil
	}
}

type ChainToolExecution struct {
	ToolName string
	Input    interface{}
	Result   interface{}
	Error    error
	Attempt  int
	Success  bool
}

type ChainToolExecutionHistory struct {
	executions []ChainToolExecution
	errorCount int
	startTime  int64
}

func NewChainToolExecutionHistory() *ChainToolExecutionHistory {
	return &ChainToolExecutionHistory{
		executions: make([]ChainToolExecution, 0),
	}
}

func (h *ChainToolExecutionHistory) AddExecution(execution ChainToolExecution) {
	h.executions = append(h.executions, execution)
	if execution.Error != nil {
		h.errorCount++
	}
}

func (h *ChainToolExecutionHistory) GetLastSuccessfulResult() interface{} {
	for i := len(h.executions) - 1; i >= 0; i-- {
		if h.executions[i].Success {
			return h.executions[i].Result
		}
	}
	return nil
}

func (h *ChainToolExecutionHistory) GetExecutionCount() int {
	return len(h.executions)
}

func (h *ChainToolExecutionHistory) GetErrorCount() int {
	return h.errorCount
}

func (h *ChainToolExecutionHistory) GetExecutionByToolName(toolName string) *ChainToolExecution {
	for i := len(h.executions) - 1; i >= 0; i-- {
		if h.executions[i].ToolName == toolName {
			return &h.executions[i]
		}
	}
	return nil
}

func (h *ChainToolExecutionHistory) Rollback(strategy RollbackStrategy) map[string]interface{} {
	results := make(map[string]interface{})

	switch strategy {
	case RollbackToStart:
		if len(h.executions) > 0 {
			first := h.executions[0]
			results[first.ToolName] = first.Input
		}

	case RollbackToPrevious:
		if len(h.executions) > 1 {
			prev := h.executions[len(h.executions)-2]
			results[prev.ToolName] = prev.Result
		}

	case RollbackSkip:
		for _, exec := range h.executions {
			if exec.Success {
				results[exec.ToolName] = exec.Result
			}
		}
	}

	return results
}

func (h *ChainToolExecutionHistory) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ChainTool Execution History: %d executions, %d errors\n", len(h.executions), h.errorCount))
	for i, exec := range h.executions {
		status := "✓"
		if !exec.Success {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("  [%d] %s %s (Attempt: %d)\n", i+1, exec.ToolName, status, exec.Attempt))
		if exec.Error != nil {
			sb.WriteString(fmt.Sprintf("       Error: %v\n", exec.Error))
		}
	}
	return sb.String()
}

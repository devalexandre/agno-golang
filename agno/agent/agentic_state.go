package agent

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// StateManagementToolkit provides tools for managing agent session state
type StateManagementToolkit struct {
	toolkit.Toolkit
	agent *Agent
}

// NewStateManagementToolkit creates a new state management toolkit for an agent
func NewStateManagementToolkit(agent *Agent) *StateManagementToolkit {
	if !agent.enableAgenticState {
		return nil
	}

	smt := &StateManagementToolkit{
		agent: agent,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "state_manager"
	tk.Description = "Manage agent session state - set, get, and delete values"

	// Register methods
	tk.Register("SetState", "Store a value in the agent's session state under a given key", smt, smt.SetState, SetStateParams{})
	tk.Register("GetState", "Retrieve a value from session state by key", smt, smt.GetState, GetStateParams{})
	tk.Register("DeleteState", "Remove a key-value pair from session state", smt, smt.DeleteState, DeleteStateParams{})
	tk.Register("ListState", "List all key-value pairs currently stored in session state", smt, smt.ListState, ListStateParams{})

	smt.Toolkit = tk

	return smt
}

// SetStateParams defines parameters for setting state
type SetStateParams struct {
	Key   string      `json:"key" jsonschema:"required,description=The state key to set"`
	Value interface{} `json:"value" jsonschema:"required,description=The value to store"`
}

// SetState sets a value in the session state
func (smt *StateManagementToolkit) SetState(params SetStateParams) (string, error) {
	if err := smt.agent.SetSessionState(params.Key, params.Value); err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully set state key '%s'", params.Key), nil
}

// GetStateParams defines parameters for getting state
type GetStateParams struct {
	Key string `json:"key" jsonschema:"required,description=The state key to retrieve"`
}

// GetState retrieves a value from the session state
func (smt *StateManagementToolkit) GetState(params GetStateParams) (interface{}, error) {
	value, ok := smt.agent.GetSessionStateValue(params.Key)
	if !ok {
		return nil, fmt.Errorf("state key '%s' not found", params.Key)
	}
	return value, nil
}

// DeleteStateParams defines parameters for deleting state
type DeleteStateParams struct {
	Key string `json:"key" jsonschema:"required,description=The state key to delete"`
}

// DeleteState removes a key from the session state
func (smt *StateManagementToolkit) DeleteState(params DeleteStateParams) (string, error) {
	if err := smt.agent.DeleteSessionState(params.Key); err != nil {
		return "", err
	}
	return fmt.Sprintf("Successfully deleted state key '%s'", params.Key), nil
}

// ListStateParams defines parameters for listing state (empty for now)
type ListStateParams struct{}

// ListState lists all keys in the session state
func (smt *StateManagementToolkit) ListState(params ListStateParams) (map[string]interface{}, error) {
	state := smt.agent.GetSessionState()
	if state == nil {
		return nil, fmt.Errorf("session state is not available")
	}
	return state, nil
}

// GetTool returns the toolkit as a Tool interface
func (smt *StateManagementToolkit) GetTool() toolkit.Tool {
	return &smt.Toolkit
}

// AgenticStateTool is a simpler wrapper that can be added to any agent
type AgenticStateTool struct {
	agent *Agent
}

// NewAgenticStateTool creates a new agentic state tool
func NewAgenticStateTool(agent *Agent) toolkit.Tool {
	if !agent.enableAgenticState {
		return nil
	}
	return NewStateManagementToolkit(agent).GetTool()
}

// WithAgenticStateContext adds session state to the context for tools to access
// This is stored in RunOptions metadata
func WithAgenticStateContext(ctx context.Context, agent *Agent) context.Context {
	if !agent.enableAgenticState {
		return ctx
	}
	return context.WithValue(ctx, "session_state", agent.sessionState)
}

// GetAgenticStateFromContext retrieves session state from context
func GetAgenticStateFromContext(ctx context.Context) (map[string]interface{}, bool) {
	state, ok := ctx.Value("session_state").(map[string]interface{})
	return state, ok
}

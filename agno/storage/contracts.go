package storage

import (
	"context"
	"time"
)

// AgentSession represents a session for an agent
type AgentSession struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	UserID      string                 `json:"user_id"`
	AgentData   map[string]interface{} `json:"agent_data"`
	UserData    map[string]interface{} `json:"user_data"`
	SessionData map[string]interface{} `json:"session_data"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AgentRun represents a single run/interaction in a session
type AgentRun struct {
	ID           string                   `json:"id"`
	SessionID    string                   `json:"session_id"`
	UserID       string                   `json:"user_id"`
	RunName      string                   `json:"run_name"`
	RunData      map[string]interface{}   `json:"run_data"`
	UserMessage  string                   `json:"user_message"`
	AgentMessage string                   `json:"agent_message"`
	Messages     []map[string]interface{} `json:"messages"`
	Metrics      map[string]interface{}   `json:"metrics"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
}

// AgentStorage defines the interface for agent storage
type AgentStorage interface {
	// Session management
	CreateSession(ctx context.Context, session *AgentSession) error
	ReadSession(ctx context.Context, sessionID string) (*AgentSession, error)
	UpdateSession(ctx context.Context, session *AgentSession) error
	DeleteSession(ctx context.Context, sessionID string) error

	// Get all sessions for a user
	GetAllSessionIDs(ctx context.Context, userID string) ([]string, error)
	GetAllSessions(ctx context.Context, userID string) ([]*AgentSession, error)

	// Run management
	CreateRun(ctx context.Context, run *AgentRun) error
	ReadRun(ctx context.Context, runID string) (*AgentRun, error)
	UpdateRun(ctx context.Context, run *AgentRun) error
	DeleteRun(ctx context.Context, runID string) error

	// Get runs for a session
	GetRunsForSession(ctx context.Context, sessionID string) ([]*AgentRun, error)

	// Schema management
	CreateTables(ctx context.Context) error
	UpgradeSchema(ctx context.Context) error
	DropTables(ctx context.Context) error
}

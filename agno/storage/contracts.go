package storage

import (
	"context"
	"time"
)

// StorageMode defines the type of storage being used
type StorageMode string

const (
	AgentMode      StorageMode = "agent"
	TeamMode       StorageMode = "team"
	WorkflowMode   StorageMode = "workflow"
	WorkflowV2Mode StorageMode = "workflow_v2"
)

// Session represents a base session compatible with Python Agno
type Session struct {
	SessionID   string                 `json:"session_id" db:"session_id"`
	UserID      string                 `json:"user_id" db:"user_id"`
	Memory      map[string]interface{} `json:"memory" db:"memory"`
	SessionData map[string]interface{} `json:"session_data" db:"session_data"`
	ExtraData   map[string]interface{} `json:"extra_data" db:"extra_data"`
	CreatedAt   int64                  `json:"created_at" db:"created_at"`
	UpdatedAt   int64                  `json:"updated_at" db:"updated_at"`
}

// AgentSession represents a session for an agent (Python compatible)
type AgentSession struct {
	Session
	AgentID       string                 `json:"agent_id" db:"agent_id"`
	AgentData     map[string]interface{} `json:"agent_data" db:"agent_data"`
	TeamSessionID *string                `json:"team_session_id,omitempty" db:"team_session_id"`
}

// TeamSession represents a session for a team (Python compatible)
type TeamSession struct {
	Session
	TeamID        string                 `json:"team_id" db:"team_id"`
	TeamData      map[string]interface{} `json:"team_data" db:"team_data"`
	TeamSessionID *string                `json:"team_session_id,omitempty" db:"team_session_id"`
}

// WorkflowSession represents a session for a workflow (Python compatible)
type WorkflowSession struct {
	Session
	WorkflowID   string                 `json:"workflow_id" db:"workflow_id"`
	WorkflowData map[string]interface{} `json:"workflow_data" db:"workflow_data"`
}

// WorkflowSessionV2 represents a session for workflow v2 (Python compatible)
type WorkflowSessionV2 struct {
	Session
	WorkflowID   string                 `json:"workflow_id" db:"workflow_id"`
	WorkflowName string                 `json:"workflow_name" db:"workflow_name"`
	WorkflowData map[string]interface{} `json:"workflow_data" db:"workflow_data"`
	Runs         map[string]interface{} `json:"runs" db:"runs"`
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

// Storage defines the universal interface for session storage (Python compatible)
type Storage interface {
	// GetID returns the unique identifier for this storage instance
	GetID() string

	// Create the storage (tables, etc.)
	Create() error

	// Read a session by ID
	Read(sessionID string, userID *string) (interface{}, error)

	// Upsert (insert or update) a session
	Upsert(session interface{}) (interface{}, error)

	// Delete a session (updated to match DB interface signature)
	DeleteSession(ctx context.Context, sessionID string) error

	// Get all session IDs for a user and optional entity (updated signature)
	GetAllSessionIDs(ctx context.Context, userID string) ([]string, error)

	// Get all sessions for a user and optional entity (updated signature)
	GetAllSessions(ctx context.Context, userID string) ([]*AgentSession, error)

	// Get recent sessions
	GetRecentSessions(userID *string, entityID *string, limit *int) ([]interface{}, error)

	// Drop the storage
	Drop() error

	// Check if table exists
	TableExists() (bool, error)

	// Upgrade schema (updated to match DB interface signature)
	UpgradeSchema(ctx context.Context) error

	// Get/Set mode
	GetMode() StorageMode
	SetMode(mode StorageMode)
}

// DB defines the interface for agent database storage (Python compatible)
// This matches Python's BaseDb interface
type DB interface {
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

// AgentStorage is an alias for DB (deprecated, use DB instead)
type AgentStorage = DB

// Helper functions to create sessions

// NewSession creates a new base session
func NewSession(sessionID, userID string) *Session {
	now := time.Now().Unix()
	return &Session{
		SessionID:   sessionID,
		UserID:      userID,
		Memory:      make(map[string]interface{}),
		SessionData: make(map[string]interface{}),
		ExtraData:   make(map[string]interface{}),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewAgentSession creates a new agent session
func NewAgentSession(sessionID, userID, agentID string) *AgentSession {
	return &AgentSession{
		Session:   *NewSession(sessionID, userID),
		AgentID:   agentID,
		AgentData: make(map[string]interface{}),
	}
}

// NewTeamSession creates a new team session
func NewTeamSession(sessionID, userID, teamID string) *TeamSession {
	return &TeamSession{
		Session:  *NewSession(sessionID, userID),
		TeamID:   teamID,
		TeamData: make(map[string]interface{}),
	}
}

// ToDict methods for serialization

// ToDict converts session to map
func (s *Session) ToDict() map[string]interface{} {
	return map[string]interface{}{
		"session_id":   s.SessionID,
		"user_id":      s.UserID,
		"memory":       s.Memory,
		"session_data": s.SessionData,
		"extra_data":   s.ExtraData,
		"created_at":   s.CreatedAt,
		"updated_at":   s.UpdatedAt,
	}
}

// ToDict converts agent session to map
func (as *AgentSession) ToDict() map[string]interface{} {
	dict := as.Session.ToDict()
	dict["agent_id"] = as.AgentID
	dict["agent_data"] = as.AgentData
	if as.TeamSessionID != nil {
		dict["team_session_id"] = *as.TeamSessionID
	}
	return dict
}

// ToDict converts team session to map
func (ts *TeamSession) ToDict() map[string]interface{} {
	dict := ts.Session.ToDict()
	dict["team_id"] = ts.TeamID
	dict["team_data"] = ts.TeamData
	if ts.TeamSessionID != nil {
		dict["team_session_id"] = *ts.TeamSessionID
	}
	return dict
}

// FromDict methods for deserialization

// FromDict creates AgentSession from map
func AgentSessionFromDict(data map[string]interface{}) *AgentSession {
	session := &AgentSession{}
	session.SessionID = getString(data, "session_id")
	session.UserID = getString(data, "user_id")
	session.AgentID = getString(data, "agent_id")
	session.Memory = getMap(data, "memory")
	session.SessionData = getMap(data, "session_data")
	session.ExtraData = getMap(data, "extra_data")
	session.AgentData = getMap(data, "agent_data")
	session.CreatedAt = getInt64(data, "created_at")
	session.UpdatedAt = getInt64(data, "updated_at")

	if teamSessionID := getString(data, "team_session_id"); teamSessionID != "" {
		session.TeamSessionID = &teamSessionID
	}

	return session
}

// FromDict creates TeamSession from map
func TeamSessionFromDict(data map[string]interface{}) *TeamSession {
	session := &TeamSession{}
	session.SessionID = getString(data, "session_id")
	session.UserID = getString(data, "user_id")
	session.TeamID = getString(data, "team_id")
	session.Memory = getMap(data, "memory")
	session.SessionData = getMap(data, "session_data")
	session.ExtraData = getMap(data, "extra_data")
	session.TeamData = getMap(data, "team_data")
	session.CreatedAt = getInt64(data, "created_at")
	session.UpdatedAt = getInt64(data, "updated_at")

	if teamSessionID := getString(data, "team_session_id"); teamSessionID != "" {
		session.TeamSessionID = &teamSessionID
	}

	return session
}

// Helper functions for type conversion
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64(data map[string]interface{}, key string) int64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getMap(data map[string]interface{}, key string) map[string]interface{} {
	if val, ok := data[key]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return make(map[string]interface{})
}

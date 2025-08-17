package memory

import (
	"context"
	"time"
)

// UserMemory represents a memory about a user
type UserMemory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Memory    string    `json:"memory"`
	Input     string    `json:"input,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionSummary represents a summary of a session
type SessionSummary struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MemoryDatabase defines the interface for memory database operations
type MemoryDatabase interface {
	// User Memory operations
	CreateUserMemory(ctx context.Context, memory *UserMemory) error
	GetUserMemories(ctx context.Context, userID string) ([]*UserMemory, error)
	UpdateUserMemory(ctx context.Context, memory *UserMemory) error
	DeleteUserMemory(ctx context.Context, memoryID string) error
	ClearUserMemories(ctx context.Context, userID string) error

	// Session Summary operations
	CreateSessionSummary(ctx context.Context, summary *SessionSummary) error
	GetSessionSummary(ctx context.Context, userID, sessionID string) (*SessionSummary, error)
	UpdateSessionSummary(ctx context.Context, summary *SessionSummary) error
	DeleteSessionSummary(ctx context.Context, userID, sessionID string) error

	// Database management
	CreateTables(ctx context.Context) error
	UpgradeSchema(ctx context.Context) error
	DropTables(ctx context.Context) error
}

// MemoryManager handles the creation and management of user memories
type MemoryManager interface {
	// Create a memory from user input and AI response
	CreateMemory(ctx context.Context, userID, input, response string) (*UserMemory, error)

	// Get all memories for a user
	GetUserMemories(ctx context.Context, userID string) ([]*UserMemory, error)

	// Update an existing memory
	UpdateMemory(ctx context.Context, memoryID, newContent string) (*UserMemory, error)

	// Delete a specific memory
	DeleteMemory(ctx context.Context, memoryID string) error

	// Clear all memories for a user
	ClearUserMemories(ctx context.Context, userID string) error

	// Create session summary
	CreateSessionSummary(ctx context.Context, userID, sessionID string, messages []map[string]interface{}) (*SessionSummary, error)

	// Get session summary
	GetSessionSummary(ctx context.Context, userID, sessionID string) (*SessionSummary, error)
}

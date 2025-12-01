package optimization

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/models"
)

// StrategyType represents the type of memory optimization strategy
type StrategyType string

const (
	StrategyTypeSummarize    StrategyType = "summarize"
	StrategyTypeRecentOnly   StrategyType = "recent_only"
	StrategyTypeKeyword      StrategyType = "keyword"
	StrategyTypeHierarchical StrategyType = "hierarchical"
)

// UserMemory represents a stored memory about a user
type UserMemory struct {
	MemoryID  string   `json:"memory_id"`
	Memory    string   `json:"memory"`
	Topics    []string `json:"topics,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
	AgentID   string   `json:"agent_id,omitempty"`
	TeamID    string   `json:"team_id,omitempty"`
	UpdatedAt int64    `json:"updated_at,omitempty"`
}

// Strategy defines the interface for memory optimization strategies
type Strategy interface {
	// Type returns the strategy type
	Type() StrategyType

	// Optimize takes a list of memories and returns optimized memories
	Optimize(ctx context.Context, memories []*UserMemory, model models.AgnoModelInterface) ([]*UserMemory, error)

	// OptimizeAsync is the async version of Optimize
	OptimizeAsync(ctx context.Context, memories []*UserMemory, model models.AgnoModelInterface) ([]*UserMemory, error)

	// GetName returns a human-readable name for the strategy
	GetName() string

	// GetDescription returns a description of what the strategy does
	GetDescription() string
}

// OptimizationResult contains the result of memory optimization
type OptimizationResult struct {
	OriginalCount  int
	OptimizedCount int
	Strategy       StrategyType
	Memories       []*UserMemory
	TokenSaved     int
}

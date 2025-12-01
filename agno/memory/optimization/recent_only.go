package optimization

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

// RecentOnlyStrategy keeps only the N most recent memories
// This is useful for focusing on recent interactions and discarding old, stale memories
type RecentOnlyStrategy struct {
	keepCount int
}

// NewRecentOnlyStrategy creates a new RecentOnlyStrategy that keeps N memories
func NewRecentOnlyStrategy(keepCount int) *RecentOnlyStrategy {
	if keepCount <= 0 {
		keepCount = 5 // Default to 5
	}
	return &RecentOnlyStrategy{
		keepCount: keepCount,
	}
}

// Type returns the strategy type
func (r *RecentOnlyStrategy) Type() StrategyType {
	return StrategyTypeRecentOnly
}

// GetName returns the strategy name
func (r *RecentOnlyStrategy) GetName() string {
	return fmt.Sprintf("Recent Only (Keep %d)", r.keepCount)
}

// GetDescription returns the strategy description
func (r *RecentOnlyStrategy) GetDescription() string {
	return fmt.Sprintf("Keeps only the %d most recent memories, discarding older ones to focus on recent interactions", r.keepCount)
}

// Optimize keeps only the N most recent memories
func (r *RecentOnlyStrategy) Optimize(ctx context.Context, memories []*UserMemory, model models.AgnoModelInterface) ([]*UserMemory, error) {
	if len(memories) == 0 {
		return []*UserMemory{}, fmt.Errorf("no memories to optimize")
	}

	// Return only the last N memories (assuming they're already sorted by recency)
	if len(memories) <= r.keepCount {
		return memories, nil
	}

	// Return the last N memories
	result := memories[len(memories)-r.keepCount:]
	return result, nil
}

// OptimizeAsync is the async version of Optimize
func (r *RecentOnlyStrategy) OptimizeAsync(ctx context.Context, memories []*UserMemory, model models.AgnoModelInterface) ([]*UserMemory, error) {
	// For now, delegate to sync version
	return r.Optimize(ctx, memories, model)
}

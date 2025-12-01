package optimization

import (
	"fmt"
)

// Factory creates strategy instances based on type
type Factory struct{}

// NewFactory creates a new strategy factory
func NewFactory() *Factory {
	return &Factory{}
}

// Create creates a strategy instance based on the type
func (f *Factory) Create(strategyType StrategyType, options ...interface{}) (Strategy, error) {
	switch strategyType {
	case StrategyTypeSummarize:
		return NewSummarizeStrategy(), nil

	case StrategyTypeRecentOnly:
		// Check if keepCount was provided
		keepCount := 5 // Default
		if len(options) > 0 {
			if count, ok := options[0].(int); ok {
				keepCount = count
			}
		}
		return NewRecentOnlyStrategy(keepCount), nil

	case StrategyTypeKeyword:
		return nil, fmt.Errorf("keyword strategy not yet implemented")

	case StrategyTypeHierarchical:
		return nil, fmt.Errorf("hierarchical strategy not yet implemented")

	default:
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}
}

// CreateByName creates a strategy by name
func (f *Factory) CreateByName(name string, options ...interface{}) (Strategy, error) {
	switch name {
	case "summarize", "Summarize":
		return NewSummarizeStrategy(), nil
	case "recent_only", "RecentOnly":
		keepCount := 5
		if len(options) > 0 {
			if count, ok := options[0].(int); ok {
				keepCount = count
			}
		}
		return NewRecentOnlyStrategy(keepCount), nil
	default:
		return nil, fmt.Errorf("unknown strategy name: %s", name)
	}
}

// ListAvailableStrategies returns a list of available strategies
func (f *Factory) ListAvailableStrategies() map[string]string {
	return map[string]string{
		"summarize":   "Combine all memories into a single comprehensive summary",
		"recent_only": "Keep only the N most recent memories",
	}
}

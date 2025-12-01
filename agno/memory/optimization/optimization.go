package optimization

// This package provides memory optimization strategies for the Agno agent
// It follows the same pattern as agno-python's memory optimization system
//
// Available strategies:
// - SummarizeStrategy: Combines multiple memories into a single comprehensive summary
// - RecentOnlyStrategy: Keeps only the N most recent memories
//
// Example usage:
//   strategy := NewSummarizeStrategy()
//   optimizedMemories, err := strategy.Optimize(ctx, memories, model)
//
// Or using the factory:
//   factory := NewFactory()
//   strategy, err := factory.Create(StrategyTypeSummarize)
//   optimizedMemories, err := strategy.Optimize(ctx, memories, model)

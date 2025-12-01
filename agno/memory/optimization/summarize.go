package optimization

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/google/uuid"
)

// SummarizeStrategy combines multiple memories into a single comprehensive summary
// This achieves maximum compression by eliminating redundancy
type SummarizeStrategy struct {
	maxTokens int
}

// NewSummarizeStrategy creates a new SummarizeStrategy instance
func NewSummarizeStrategy() *SummarizeStrategy {
	return &SummarizeStrategy{
		maxTokens: 2000,
	}
}

// Type returns the strategy type
func (s *SummarizeStrategy) Type() StrategyType {
	return StrategyTypeSummarize
}

// GetName returns the strategy name
func (s *SummarizeStrategy) GetName() string {
	return "Summarize"
}

// GetDescription returns the strategy description
func (s *SummarizeStrategy) GetDescription() string {
	return "Combines all memories into a single comprehensive summary, achieving maximum compression by eliminating redundancy"
}

// Optimize combines all memories into a single summary
func (s *SummarizeStrategy) Optimize(ctx context.Context, memories []*UserMemory, model models.AgnoModelInterface) ([]*UserMemory, error) {
	if len(memories) == 0 {
		return []*UserMemory{}, fmt.Errorf("no memories to optimize")
	}

	// Collect all memory contents
	var memoryContents []string
	var allTopics []string
	userID := ""
	agentID := ""
	teamID := ""

	for _, mem := range memories {
		if mem.Memory != "" {
			memoryContents = append(memoryContents, mem.Memory)
		}
		if mem.Topics != nil {
			allTopics = append(allTopics, mem.Topics...)
		}
		if userID == "" && mem.UserID != "" {
			userID = mem.UserID
		}
		if agentID == "" && mem.AgentID != "" {
			agentID = mem.AgentID
		}
		if teamID == "" && mem.TeamID != "" {
			teamID = mem.TeamID
		}
	}

	if len(memoryContents) == 0 {
		return []*UserMemory{}, fmt.Errorf("no memory content to summarize")
	}

	// Combine memory contents
	combinedContent := strings.Join(memoryContents, "\n\n")

	// Create summarization prompt
	systemPrompt := s.getSystemPrompt()
	userPrompt := fmt.Sprintf("Summarize these memories into a single summary:\n\n%s", combinedContent)

	// Call model to generate summary
	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: systemPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: userPrompt,
		},
	}

	response, err := model.Invoke(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	summarizedContent := strings.TrimSpace(response.Content)

	// Deduplicate topics
	uniqueTopics := deduplicateTopics(allTopics)

	// Create summarized memory
	summarizedMemory := &UserMemory{
		MemoryID:  uuid.New().String(),
		Memory:    summarizedContent,
		Topics:    uniqueTopics,
		UserID:    userID,
		AgentID:   agentID,
		TeamID:    teamID,
		UpdatedAt: getCurrentTimestamp(),
	}

	return []*UserMemory{summarizedMemory}, nil
}

// OptimizeAsync is the async version of Optimize
func (s *SummarizeStrategy) OptimizeAsync(ctx context.Context, memories []*UserMemory, model models.AgnoModelInterface) ([]*UserMemory, error) {
	// For now, delegate to sync version
	// In production, this would be truly async
	return s.Optimize(ctx, memories, model)
}

// getSystemPrompt returns the system prompt for summarization
func (s *SummarizeStrategy) getSystemPrompt() string {
	return `You are a memory compression assistant. Your task is to summarize multiple memories about a user into a single comprehensive summary while preserving all key facts.

Requirements:
- Combine related information from all memories
- Preserve all factual information
- Remove redundancy and consolidate repeated facts
- Create a coherent narrative about the user
- Maintain third-person perspective
- Do not add information not present in the original memories

Return only the summarized memory text, nothing else.`
}

// deduplicateTopics removes duplicate topics
func deduplicateTopics(topics []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, topic := range topics {
		if !seen[topic] && topic != "" {
			seen[topic] = true
			result = append(result, topic)
		}
	}

	return result
}

// getCurrentTimestamp returns the current Unix timestamp in seconds
func getCurrentTimestamp() int64 {
	return 0 // Placeholder - would use time.Now().Unix() in real implementation
}

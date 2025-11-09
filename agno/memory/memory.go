package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/models"
)

// Memory implements the MemoryManager interface
type Memory struct {
	Model    models.AgnoModelInterface
	DB       MemoryDatabase
	Embedder embedder.Embedder // Optional: for semantic search
}

// NewMemory creates a new Memory instance
func NewMemory(model models.AgnoModelInterface, db MemoryDatabase) *Memory {
	return &Memory{
		Model: model,
		DB:    db,
	}
}

// NewMemoryWithEmbedder creates a new Memory instance with embedder for semantic search
func NewMemoryWithEmbedder(model models.AgnoModelInterface, db MemoryDatabase, emb embedder.Embedder) *Memory {
	return &Memory{
		Model:    model,
		DB:       db,
		Embedder: emb,
	}
}

// CreateMemory creates a memory from user input and AI response
func (m *Memory) CreateMemory(ctx context.Context, userID, input, response string) (*UserMemory, error) {
	// Use AI to extract meaningful information from the conversation
	memoryContent, err := m.extractMemoryFromConversation(ctx, input, response)
	if err != nil {
		return nil, fmt.Errorf("failed to extract memory: %w", err)
	}

	// Skip empty memories
	if strings.TrimSpace(memoryContent) == "" {
		return nil, nil
	}

	memory := &UserMemory{
		UserID:  userID,
		Memory:  memoryContent,
		Input:   input,
		Summary: "", // Could be generated later if needed
	}

	err = m.DB.CreateUserMemory(ctx, memory)
	if err != nil {
		return nil, fmt.Errorf("failed to save memory: %w", err)
	}

	return memory, nil
}

// GetUserMemories gets all memories for a user
func (m *Memory) GetUserMemories(ctx context.Context, userID string) ([]*UserMemory, error) {
	return m.DB.GetUserMemories(ctx, userID)
}

// UpdateMemory updates an existing memory
func (m *Memory) UpdateMemory(ctx context.Context, memoryID, newContent string) (*UserMemory, error) {
	// We need to implement a way to get memory by ID first
	// This is a simplified implementation
	memories, err := m.DB.GetUserMemories(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, memory := range memories {
		if memory.ID == memoryID {
			memory.Memory = newContent
			err = m.DB.UpdateUserMemory(ctx, memory)
			if err != nil {
				return nil, fmt.Errorf("failed to update memory: %w", err)
			}
			return memory, nil
		}
	}

	return nil, fmt.Errorf("memory not found: %s", memoryID)
}

// DeleteMemory deletes a specific memory
func (m *Memory) DeleteMemory(ctx context.Context, memoryID string) error {
	return m.DB.DeleteUserMemory(ctx, memoryID)
}

// ClearUserMemories clears all memories for a user
func (m *Memory) ClearUserMemories(ctx context.Context, userID string) error {
	return m.DB.ClearUserMemories(ctx, userID)
}

// CreateSessionSummary creates a session summary
func (m *Memory) CreateSessionSummary(ctx context.Context, userID, sessionID string, messages []map[string]interface{}) (*SessionSummary, error) {
	// Generate summary using AI
	summaryContent, err := m.generateSessionSummary(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session summary: %w", err)
	}

	summary := &SessionSummary{
		UserID:    userID,
		SessionID: sessionID,
		Summary:   summaryContent,
	}

	err = m.DB.CreateSessionSummary(ctx, summary)
	if err != nil {
		return nil, fmt.Errorf("failed to save session summary: %w", err)
	}

	return summary, nil
}

// GetSessionSummary gets a session summary
func (m *Memory) GetSessionSummary(ctx context.Context, userID, sessionID string) (*SessionSummary, error) {
	return m.DB.GetSessionSummary(ctx, userID, sessionID)
}

// extractMemoryFromConversation uses AI to extract meaningful information
func (m *Memory) extractMemoryFromConversation(ctx context.Context, input, response string) (string, error) {
	prompt := fmt.Sprintf(`Analyze the following conversation between a user and an AI assistant.
Extract any important facts, preferences, or personal information about the user that should be remembered for future interactions.

Guidelines:
- Only extract factual information about the user
- Include preferences, hobbies, work, personal details, etc.
- Keep it concise and clear
- If there's nothing meaningful to remember, return empty
- Do not include temporary information like current weather or time

User: %s
Assistant: %s

Important information to remember about the user:`, input, response)

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: "You are a memory extraction assistant. Extract important facts about users from conversations.",
		},
		{
			Role:    models.TypeUserRole,
			Content: prompt,
		},
	}

	response_, err := m.Model.Invoke(ctx, messages)
	if err != nil {
		return "", err
	}

	memory := strings.TrimSpace(response_.Content)

	// Filter out common non-memory responses
	lowerMemory := strings.ToLower(memory)
	if strings.Contains(lowerMemory, "nothing meaningful") ||
		strings.Contains(lowerMemory, "no important") ||
		strings.Contains(lowerMemory, "no specific") ||
		len(memory) < 10 {
		return "", nil
	}

	return memory, nil
}

// generateSessionSummary creates a summary of the session
func (m *Memory) generateSessionSummary(ctx context.Context, messages []map[string]interface{}) (string, error) {
	// Convert messages to a readable format
	var conversation strings.Builder
	for _, msg := range messages {
		role, ok := msg["role"].(string)
		if !ok {
			continue
		}
		content, ok := msg["content"].(string)
		if !ok {
			continue
		}

		conversation.WriteString(fmt.Sprintf("%s: %s\n", role, content))
	}

	prompt := fmt.Sprintf(`Summarize the following conversation between a user and an AI assistant.
Create a concise summary that captures:
- The main topics discussed
- Key questions asked by the user
- Important decisions or conclusions reached
- Any action items or next steps

Keep the summary under 200 words and focus on the most important aspects.

Conversation:
%s

Summary:`, conversation.String())

	aiMessages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: "You are a conversation summarization assistant. Create concise, informative summaries.",
		},
		{
			Role:    models.TypeUserRole,
			Content: prompt,
		},
	}

	response, err := m.Model.Invoke(ctx, aiMessages)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response.Content), nil
}

// GetMemoriesAsContext returns user memories formatted for AI context
func (m *Memory) GetMemoriesAsContext(ctx context.Context, userID string) (string, error) {
	memories, err := m.GetUserMemories(ctx, userID)
	if err != nil {
		return "", err
	}

	if len(memories) == 0 {
		return "", nil
	}

	var context strings.Builder
	context.WriteString("What I know about this user:\n")

	for _, memory := range memories {
		context.WriteString(fmt.Sprintf("- %s\n", memory.Memory))
	}

	return context.String(), nil
}

// SearchMemoriesSemantic performs semantic search on user memories
// Returns memories ranked by relevance to the query
func (m *Memory) SearchMemoriesSemantic(ctx context.Context, userID, query string, limit int) ([]*UserMemory, error) {
	if m.Embedder == nil {
		return nil, fmt.Errorf("embedder not configured for semantic search")
	}

	// Get all user memories
	allMemories, err := m.GetUserMemories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memories: %w", err)
	}

	if len(allMemories) == 0 {
		return []*UserMemory{}, nil
	}

	// Generate embedding for the query
	queryEmbedding, err := m.Embedder.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Calculate similarity scores for each memory
	type scoredMemory struct {
		memory *UserMemory
		score  float64
	}

	scoredMemories := make([]scoredMemory, 0, len(allMemories))

	for _, memory := range allMemories {
		// Generate embedding for the memory
		memoryEmbedding, err := m.Embedder.GetEmbedding(memory.Memory)
		if err != nil {
			// Skip memories that fail to embed
			continue
		}

		// Calculate cosine similarity
		similarity := cosineSimilarity(queryEmbedding, memoryEmbedding)

		scoredMemories = append(scoredMemories, scoredMemory{
			memory: memory,
			score:  similarity,
		})
	}

	// Sort by similarity score (descending)
	sort.Slice(scoredMemories, func(i, j int) bool {
		return scoredMemories[i].score > scoredMemories[j].score
	})

	// Return top N results
	if limit <= 0 || limit > len(scoredMemories) {
		limit = len(scoredMemories)
	}

	results := make([]*UserMemory, limit)
	for i := 0; i < limit; i++ {
		results[i] = scoredMemories[i].memory
	}

	return results, nil
}

// SearchMemoriesHybrid performs hybrid search combining semantic and keyword matching
func (m *Memory) SearchMemoriesHybrid(ctx context.Context, userID, query string, limit int, semanticWeight float64) ([]*UserMemory, error) {
	if m.Embedder == nil {
		// Fall back to keyword search if no embedder
		return m.SearchMemoriesKeyword(ctx, userID, query, limit)
	}

	// Get all user memories
	allMemories, err := m.GetUserMemories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memories: %w", err)
	}

	if len(allMemories) == 0 {
		return []*UserMemory{}, nil
	}

	// Generate embedding for the query
	queryEmbedding, err := m.Embedder.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Normalize semantic weight
	if semanticWeight < 0 {
		semanticWeight = 0
	}
	if semanticWeight > 1 {
		semanticWeight = 1
	}
	keywordWeight := 1.0 - semanticWeight

	// Calculate hybrid scores
	type scoredMemory struct {
		memory *UserMemory
		score  float64
	}

	queryLower := strings.ToLower(query)
	scoredMemories := make([]scoredMemory, 0, len(allMemories))

	for _, memory := range allMemories {
		// Semantic score
		var semanticScore float64
		memoryEmbedding, err := m.Embedder.GetEmbedding(memory.Memory)
		if err == nil {
			semanticScore = cosineSimilarity(queryEmbedding, memoryEmbedding)
		}

		// Keyword score (simple TF-IDF-like scoring)
		keywordScore := calculateKeywordScore(queryLower, strings.ToLower(memory.Memory))

		// Combine scores
		hybridScore := (semanticScore * semanticWeight) + (keywordScore * keywordWeight)

		scoredMemories = append(scoredMemories, scoredMemory{
			memory: memory,
			score:  hybridScore,
		})
	}

	// Sort by hybrid score (descending)
	sort.Slice(scoredMemories, func(i, j int) bool {
		return scoredMemories[i].score > scoredMemories[j].score
	})

	// Return top N results
	if limit <= 0 || limit > len(scoredMemories) {
		limit = len(scoredMemories)
	}

	results := make([]*UserMemory, limit)
	for i := 0; i < limit; i++ {
		results[i] = scoredMemories[i].memory
	}

	return results, nil
}

// SearchMemoriesKeyword performs simple keyword-based search
func (m *Memory) SearchMemoriesKeyword(ctx context.Context, userID, query string, limit int) ([]*UserMemory, error) {
	allMemories, err := m.GetUserMemories(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user memories: %w", err)
	}

	if len(allMemories) == 0 {
		return []*UserMemory{}, nil
	}

	queryLower := strings.ToLower(query)

	type scoredMemory struct {
		memory *UserMemory
		score  float64
	}

	scoredMemories := make([]scoredMemory, 0)

	for _, memory := range allMemories {
		score := calculateKeywordScore(queryLower, strings.ToLower(memory.Memory))
		if score > 0 {
			scoredMemories = append(scoredMemories, scoredMemory{
				memory: memory,
				score:  score,
			})
		}
	}

	// Sort by score (descending)
	sort.Slice(scoredMemories, func(i, j int) bool {
		return scoredMemories[i].score > scoredMemories[j].score
	})

	// Return top N results
	if limit <= 0 || limit > len(scoredMemories) {
		limit = len(scoredMemories)
	}

	results := make([]*UserMemory, limit)
	for i := 0; i < limit; i++ {
		results[i] = scoredMemories[i].memory
	}

	return results, nil
}

// Helper functions

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64

	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// calculateKeywordScore calculates a simple keyword matching score
func calculateKeywordScore(query, text string) float64 {
	queryWords := strings.Fields(query)
	if len(queryWords) == 0 {
		return 0
	}

	matches := 0
	for _, word := range queryWords {
		if strings.Contains(text, word) {
			matches++
		}
	}

	// Return ratio of matched words
	return float64(matches) / float64(len(queryWords))
}

package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Memory implements the MemoryManager interface
type Memory struct {
	Model models.AgnoModelInterface
	DB    MemoryDatabase
}

// NewMemory creates a new Memory instance
func NewMemory(model models.AgnoModelInterface, db MemoryDatabase) *Memory {
	return &Memory{
		Model: model,
		DB:    db,
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

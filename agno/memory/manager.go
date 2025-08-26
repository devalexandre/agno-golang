package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/google/uuid"
)

// EnhancedMemoryManager extends the basic Memory with advanced features
type EnhancedMemoryManager struct {
	*Memory
	Classifier        *MemoryClassifier
	Summarizer        *MemorySummarizer
	Limit             *int
	SystemPrompt      *string
	InputMessage      *string
	ToolsForModel     []map[string]interface{}
	FunctionsForModel map[string]interface{}
}

// NewEnhancedMemoryManager creates a new EnhancedMemoryManager instance
func NewEnhancedMemoryManager(model models.AgnoModelInterface, db MemoryDatabase) *EnhancedMemoryManager {
	basicMemory := NewMemory(model, db)
	classifier := NewMemoryClassifier(model)
	summarizer := NewMemorySummarizer(model)

	return &EnhancedMemoryManager{
		Memory:     basicMemory,
		Classifier: classifier,
		Summarizer: summarizer,
	}
}

// UpdateModel updates the model with defaults
func (emm *EnhancedMemoryManager) UpdateModel() {
	// Use the default Model (OpenAIChat) if no model is provided
	if emm.Model == nil {
		// In a real implementation, we would create a default model here
		// For now, we'll assume the model is already set
	}
}

// GetExistingMemories retrieves existing memories for the user
func (emm *EnhancedMemoryManager) GetExistingMemories(ctx context.Context, userID string) ([]*UserMemory, error) {
	if emm.DB == nil {
		return nil, nil
	}

	return emm.DB.GetUserMemories(ctx, userID)
}

// ShouldUpdateMemory determines if a message should be added to memory
func (emm *EnhancedMemoryManager) ShouldUpdateMemory(ctx context.Context, userID, input string) (bool, error) {
	existingMemories, err := emm.GetExistingMemories(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get existing memories: %w", err)
	}

	emm.Classifier.ExistingMemories = existingMemories
	return emm.Classifier.ShouldUpdateMemory(ctx, input)
}

// AddMemory adds a memory to the database
func (emm *EnhancedMemoryManager) AddMemory(ctx context.Context, userID, memoryContent, input string) (*UserMemory, error) {
	if emm.DB == nil {
		return nil, fmt.Errorf("memory database not provided")
	}

	memory := &UserMemory{
		ID:        uuid.New().String(),
		UserID:    userID,
		Memory:    memoryContent,
		Input:     input,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := emm.DB.CreateUserMemory(ctx, memory)
	if err != nil {
		return nil, fmt.Errorf("failed to store memory in db: %w", err)
	}

	return memory, nil
}

// DeleteMemory deletes a memory from the database
func (emm *EnhancedMemoryManager) DeleteMemory(ctx context.Context, memoryID string) error {
	if emm.DB == nil {
		return fmt.Errorf("memory database not provided")
	}

	err := emm.DB.DeleteUserMemory(ctx, memoryID)
	if err != nil {
		return fmt.Errorf("failed to delete memory from db: %w", err)
	}

	return nil
}

// UpdateMemory updates a memory in the database
func (emm *EnhancedMemoryManager) UpdateMemory(ctx context.Context, memoryID, userID, memoryContent, input string) (*UserMemory, error) {
	if emm.DB == nil {
		return nil, fmt.Errorf("memory database not provided")
	}

	memory := &UserMemory{
		ID:        memoryID,
		UserID:    userID,
		Memory:    memoryContent,
		Input:     input,
		UpdatedAt: time.Now(),
	}

	err := emm.DB.UpdateUserMemory(ctx, memory)
	if err != nil {
		return nil, fmt.Errorf("failed to update memory in db: %w", err)
	}

	return memory, nil
}

// ClearMemory clears all memories from the database
func (emm *EnhancedMemoryManager) ClearMemory(ctx context.Context, userID string) error {
	if emm.DB == nil {
		return fmt.Errorf("memory database not provided")
	}

	err := emm.DB.ClearUserMemories(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to clear memory from db: %w", err)
	}

	return nil
}

// GetSystemMessage returns a system message for the memory manager
func (emm *EnhancedMemoryManager) GetSystemMessage(ctx context.Context, userID string) (models.Message, error) {
	// Return a system message for the memory manager
	systemPromptLines := []string{
		"Your task is to generate a concise memory for the user's message. ",
		"Create a memory that captures the key information provided by the user, as if you were storing it for future reference. ",
		"The memory should be a brief, third-person statement that encapsulates the most important aspect of the user's input, without adding any extraneous details. ",
		"This memory will be used to enhance the user's experience in subsequent conversations.",
		"You will also be provided with a list of existing memories. You may:",
		"  1. Add a new memory using the `add_memory` function.",
		"  2. Update a memory using the `update_memory` function.",
		"  3. Delete a memory using the `delete_memory` function.",
		"  4. Clear all memories using the `clear_memory` function. Use this with extreme caution, as it will remove all memories from the database.",
	}

	existingMemories, err := emm.GetExistingMemories(ctx, userID)
	if err == nil && len(existingMemories) > 0 {
		systemPromptLines = append(systemPromptLines, "\nExisting memories:")
		var memoriesBuilder strings.Builder
		memoriesBuilder.WriteString("<existing_memories>\n")
		for _, m := range existingMemories {
			memoriesBuilder.WriteString(fmt.Sprintf("  - id: %s | memory: %s\n", m.ID, m.Memory))
		}
		memoriesBuilder.WriteString("</existing_memories>")
		systemPromptLines = append(systemPromptLines, memoriesBuilder.String())
	}

	return models.Message{
		Role:    models.TypeSystemRole,
		Content: strings.Join(systemPromptLines, "\n"),
	}, nil
}

// Run processes a message and manages memories
func (emm *EnhancedMemoryManager) Run(ctx context.Context, userID, message string) (*UserMemory, error) {
	// Update the Model (set defaults, add logit etc.)
	emm.UpdateModel()

	// Check if this user message should be added to long term memory
	shouldUpdateMemory, err := emm.ShouldUpdateMemory(ctx, userID, message)
	if err != nil {
		return nil, fmt.Errorf("failed to determine if memory should be updated: %w", err)
	}

	if !shouldUpdateMemory {
		return nil, nil // No memory to create
	}

	// Create memory from the message
	memory, err := emm.AddMemory(ctx, userID, message, message)
	if err != nil {
		return nil, fmt.Errorf("failed to add memory: %w", err)
	}

	return memory, nil
}

// CreateSessionSummary creates a summary of the session
func (emm *EnhancedMemoryManager) CreateSessionSummary(ctx context.Context, userID, sessionID string, messagePairs []MessagePair) (*SessionSummary, error) {
	summary, err := emm.Summarizer.CreateSessionSummary(ctx, userID, sessionID, messagePairs)
	if err != nil {
		return nil, fmt.Errorf("failed to create session summary: %w", err)
	}

	// Save the summary to the database
	if emm.DB != nil {
		err = emm.DB.CreateSessionSummary(ctx, summary)
		if err != nil {
			return nil, fmt.Errorf("failed to save session summary: %w", err)
		}
	}

	return summary, nil
}

// GetSessionSummary retrieves a session summary
func (emm *EnhancedMemoryManager) GetSessionSummary(ctx context.Context, userID, sessionID string) (*SessionSummary, error) {
	if emm.DB == nil {
		return nil, fmt.Errorf("memory database not provided")
	}

	return emm.DB.GetSessionSummary(ctx, userID, sessionID)
}

// GetMemoriesAsContext returns user memories formatted for AI context
func (emm *EnhancedMemoryManager) GetMemoriesAsContext(ctx context.Context, userID string) (string, error) {
	memories, err := emm.GetUserMemories(ctx, userID)
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

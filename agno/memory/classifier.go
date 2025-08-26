package memory

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

// MemoryClassifier determines if a message should be stored as a memory
type MemoryClassifier struct {
	Model            models.AgnoModelInterface
	SystemPrompt     string
	ExistingMemories []*UserMemory
}

// NewMemoryClassifier creates a new MemoryClassifier instance
func NewMemoryClassifier(model models.AgnoModelInterface) *MemoryClassifier {
	return &MemoryClassifier{
		Model: model,
	}
}

// GetSystemMessage returns the system message for classification
func (mc *MemoryClassifier) GetSystemMessage() models.Message {
	systemPromptLines := []string{
		"Your task is to identify if the user's message contains information that is worth remembering for future conversations.",
		"This includes details that could personalize ongoing interactions with the user, such as:",
		"  - Personal facts: name, age, occupation, location, interests, preferences, etc.",
		"  - Significant life events or experiences shared by the user",
		"  - Important context about the user's current situation, challenges or goals",
		"  - What the user likes or dislikes, their opinions, beliefs, values, etc.",
		"  - Any other details that provide valuable insights into the user's personality, perspective or needs",
		"Your task is to decide whether the user input contains any of the above information worth remembering.",
		"If the user input contains any information worth remembering for future conversations, respond with 'yes'.",
		"If the input does not contain any important details worth saving, respond with 'no' to disregard it.",
		"You will also be provided with a list of existing memories to help you decide if the input is new or already known.",
		"If the memory already exists that matches the input, respond with 'no' to keep it as is.",
		"If a memory exists that needs to be updated or deleted, respond with 'yes' to update/delete it.",
		"You must only respond with 'yes' or 'no'. Nothing else will be considered as a valid response.",
	}

	if mc.ExistingMemories != nil && len(mc.ExistingMemories) > 0 {
		systemPromptLines = append(systemPromptLines, "\nExisting memories:")
		var memoriesBuilder strings.Builder
		memoriesBuilder.WriteString("<existing_memories>\n")
		for _, m := range mc.ExistingMemories {
			memoriesBuilder.WriteString(fmt.Sprintf("  - %s\n", m.Memory))
		}
		memoriesBuilder.WriteString("</existing_memories>")
		systemPromptLines = append(systemPromptLines, memoriesBuilder.String())
	}

	return models.Message{
		Role:    models.TypeSystemRole,
		Content: strings.Join(systemPromptLines, "\n"),
	}
}

// ShouldUpdateMemory determines if a message should be added to memory
func (mc *MemoryClassifier) ShouldUpdateMemory(ctx context.Context, input string) (bool, error) {
	if mc.SystemPrompt == "" {
		mc.SystemPrompt = mc.GetSystemMessage().Content
	}

	messages := []models.Message{
		{
			Role:    models.TypeSystemRole,
			Content: mc.SystemPrompt,
		},
		{
			Role:    models.TypeUserRole,
			Content: input,
		},
	}

	response, err := mc.Model.Invoke(ctx, messages)
	if err != nil {
		return false, fmt.Errorf("failed to classify memory: %w", err)
	}

	classification := strings.TrimSpace(strings.ToLower(response.Content))
	return classification == "yes", nil
}

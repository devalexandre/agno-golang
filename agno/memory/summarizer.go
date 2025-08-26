package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

// MemorySummarizer creates summaries of conversations
type MemorySummarizer struct {
	Model                models.AgnoModelInterface
	UseStructuredOutputs bool
}

// NewMemorySummarizer creates a new MemorySummarizer instance
func NewMemorySummarizer(model models.AgnoModelInterface) *MemorySummarizer {
	return &MemorySummarizer{
		Model: model,
	}
}

// GetSystemMessage returns the system message for summarization
func (ms *MemorySummarizer) GetSystemMessage(messagesForSummarization []map[string]string) models.Message {
	systemPromptLines := []string{
		"Analyze the following conversation between a user and an assistant.",
		"Create a concise summary that captures:",
		"- The main topics discussed",
		"- Key questions asked by the user",
		"- Important decisions or conclusions reached",
		"- Any action items or next steps",
		"",
		"Keep the summary under 200 words and focus on the most important aspects.",
		"",
		"Conversation:",
	}

	var conversationBuilder strings.Builder
	for _, messagePair := range messagesForSummarization {
		if userMsg, ok := messagePair["user"]; ok {
			conversationBuilder.WriteString(fmt.Sprintf("User: %s\n", userMsg))
		}
		if assistantMsg, ok := messagePair["assistant"]; ok {
			conversationBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", assistantMsg))
		} else if modelMsg, ok := messagePair["model"]; ok {
			conversationBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", modelMsg))
		}
	}

	systemPromptLines = append(systemPromptLines, conversationBuilder.String())

	if !ms.UseStructuredOutputs {
		systemPromptLines = append(systemPromptLines, "Provide your output as a JSON containing the following field:")
		systemPromptLines = append(systemPromptLines, "\"summary\": \"The conversation summary\"")
		systemPromptLines = append(systemPromptLines, "Start your response with `{` and end it with `}`.")
		systemPromptLines = append(systemPromptLines, "Your output will be passed to json.Unmarshal to convert it to a Go struct.")
		systemPromptLines = append(systemPromptLines, "Make sure it only contains valid JSON.")
	}

	return models.Message{
		Role:    models.TypeSystemRole,
		Content: strings.Join(systemPromptLines, "\n"),
	}
}

// CreateSessionSummary creates a summary of the session
func (ms *MemorySummarizer) CreateSessionSummary(ctx context.Context, userID, sessionID string, messagePairs []MessagePair) (*SessionSummary, error) {
	if len(messagePairs) == 0 {
		return nil, fmt.Errorf("no message pairs provided for summarization")
	}

	// Convert the message pairs to a list of dictionaries
	messagesForSummarization := make([]map[string]string, 0, len(messagePairs))
	for _, messagePair := range messagePairs {
		messageDict := map[string]string{
			"user": messagePair.UserMessage.Content,
		}

		if messagePair.AssistantMessage.Content != "" {
			messageDict["assistant"] = messagePair.AssistantMessage.Content
		} else if messagePair.ModelMessage.Content != "" {
			messageDict["model"] = messagePair.ModelMessage.Content
		}

		messagesForSummarization = append(messagesForSummarization, messageDict)
	}

	// Prepare the List of messages to send to the Model
	messagesForModel := []models.Message{
		ms.GetSystemMessage(messagesForSummarization),
		// For models that require a non-system message
		{
			Role:    models.TypeUserRole,
			Content: "Provide the summary of the conversation.",
		},
	}

	// Set response format if it is not set on the Model
	var responseFormat interface{}
	if ms.UseStructuredOutputs {
		responseFormat = "json_object"
	} else {
		responseFormat = map[string]string{"type": "json_object"}
	}

	// Generate a response from the Model
	response, err := ms.Model.Invoke(ctx, messagesForModel, models.WithResponseFormat(responseFormat))
	if err != nil {
		return nil, fmt.Errorf("failed to generate session summary: %w", err)
	}

	// Parse the response
	if response.Content != "" {
		// Extract summary from JSON response
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(response.Content), &jsonResponse); err != nil {
			// Try to extract JSON from markdown code blocks
			content := strings.TrimSpace(response.Content)
			if strings.HasPrefix(content, "```json") {
				content = strings.TrimPrefix(content, "```json")
				content = strings.TrimSuffix(content, "```")
				content = strings.TrimSpace(content)

				if err := json.Unmarshal([]byte(content), &jsonResponse); err != nil {
					return nil, fmt.Errorf("failed to parse session summary response: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to parse session summary response: %w", err)
			}
		}

		// Extract summary from JSON
		summaryText, ok := jsonResponse["summary"].(string)
		if !ok {
			// If no summary field, use the entire content
			summaryText = response.Content
		}

		sessionSummary := &SessionSummary{
			UserID:    userID,
			SessionID: sessionID,
			Summary:   summaryText,
		}

		return sessionSummary, nil
	}

	return nil, fmt.Errorf("empty response from model")
}

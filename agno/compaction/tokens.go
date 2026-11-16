package compaction

import (
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

// EstimateTextTokens is the local fallback heuristic from Python's
// agno.utils.tokens.count_text_tokens: about 4 characters per token.
func EstimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

// EstimateMessageTokens estimates the tokens of one message: its content,
// tool call arguments, tool call id and reasoning are joined and counted as
// text, plus a flat cost for attached media. It never makes a network call.
func EstimateMessageTokens(m models.Message) int {
	parts := make([]string, 0, 4)
	if m.Content != "" {
		parts = append(parts, m.Content)
	}
	for _, tc := range m.ToolCalls {
		if tc.Function.Arguments != "" {
			parts = append(parts, tc.Function.Arguments)
		}
	}
	if m.ToolCallID != nil && *m.ToolCallID != "" {
		parts = append(parts, *m.ToolCallID)
	}
	if m.Thinking != "" {
		parts = append(parts, m.Thinking)
	}
	tokens := EstimateTextTokens(strings.Join(parts, " "))
	tokens += mediaTokens(m)
	return tokens
}

// mediaTokens approximates media cost. Python counts image tiles from the
// image dimensions; without decoding images here, each image is charged the
// 85-token base, and file payloads are charged like Python's binary fallback.
func mediaTokens(m models.Message) int {
	tokens := 85 * len(m.Images)
	for _, f := range m.Files {
		if strings.HasPrefix(f.MimeType, "text/") {
			tokens += len(f.Data) / 4
		} else {
			tokens += len(f.Data) / 40
		}
	}
	return tokens
}

// EstimateTokens estimates the tokens of a list of messages.
func EstimateTokens(messages []models.Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateMessageTokens(m)
	}
	return total
}

// countRequest counts with the configured TokenCounter, falling back to the
// local estimate when there is none or it returns a negative number.
func (c Config) countRequest(messages []models.Message) int {
	if c.TokenCounter != nil {
		if n := c.TokenCounter(messages); n >= 0 {
			return n
		}
	}
	return EstimateTokens(messages)
}

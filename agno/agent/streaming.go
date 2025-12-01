package agent

import (
	"context"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/utils"
)

// runWithStreaming executes the agent with streaming UI and returns the response
func (a *Agent) runWithStreaming(prompt string, messages []models.Message) (*models.MessageResponse, error) {
	start := time.Now()

	// Thinking
	spinnerResponse := utils.ThinkingPanel(prompt)
	contentChan := utils.StartSimplePanel(spinnerResponse, start, true) // Always use markdown for CLI

	// Response
	fullResponse := ""
	var streamBuffer string
	showResponse := false

	callOptions := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			if !showResponse {
				showResponse = true
			}

			// Add chunk to buffer
			streamBuffer += string(chunk)
			fullResponse += string(chunk)

			// Check if we should flush buffer
			shouldFlush := false

			// Flush if finding period, exclamation or question mark
			if strings.Contains(streamBuffer, ".") ||
				strings.Contains(streamBuffer, "!") ||
				strings.Contains(streamBuffer, "?") {
				shouldFlush = true
			}

			// Flush if buffer gets too large
			if len(streamBuffer) > 50 {
				shouldFlush = true
			}

			if shouldFlush {
				// Send accumulated content
				contentChan <- utils.ContentUpdateMsg{
					PanelName: "Response",
					Content:   streamBuffer,
				}
				streamBuffer = "" // Clear buffer
			}

			return nil
		}),
	}

	err := a.model.InvokeStream(a.ctx, messages, callOptions...)

	// Flush any remaining content in buffer
	if streamBuffer != "" {
		contentChan <- utils.ContentUpdateMsg{
			PanelName: "Response",
			Content:   streamBuffer,
		}
	}

	// Close channel to stop the streaming goroutine
	close(contentChan)

	// Wait a bit for UI to finish rendering
	time.Sleep(100 * time.Millisecond)

	if err != nil {
		return nil, err
	}

	// Construct response object
	return &models.MessageResponse{
		Model:   a.model.GetID(), // Assuming GetID exists or we use a.model.ID if available
		Role:    "assistant",
		Content: fullResponse,
		// Note: We might miss some metrics here as InvokeStream doesn't return them directly
	}, nil
}

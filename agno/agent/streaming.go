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

	// Show prompt
	utils.PromptPanel(prompt)

	// Start streaming panel for thinking and response
	contentChan := utils.StartSimplePanel(nil, start, true) // Always use markdown for CLI

	// Response
	fullResponse := ""
	var streamBuffer string
	showResponse := false
	thinkingContent := strings.Builder{}
	responseContent := strings.Builder{}
	inThinkingTag := false

	callOptions := []models.Option{
		models.WithTools(a.tools),
		models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			if !showResponse {
				showResponse = true
			}

			chunkStr := string(chunk)
			fullResponse += chunkStr

			// Detect <think> tags
			if strings.Contains(chunkStr, "<think>") {
				inThinkingTag = true
				// Send what we have as thinking
				parts := strings.Split(chunkStr, "<think>")
				if len(parts) > 1 {
					chunkStr = parts[1]
					thinkingContent.WriteString(chunkStr)

					// Send initial thinking content
					contentChan <- utils.ContentUpdateMsg{
						PanelName: utils.MessageThinking,
						Content:   chunkStr,
						Replace:   false,
					}
				}
				return nil
			}

			if inThinkingTag {
				if strings.Contains(chunkStr, "</think>") {
					// End of thinking block
					parts := strings.Split(chunkStr, "</think>")
					if len(parts) > 0 && parts[0] != "" {
						thinkingContent.WriteString(parts[0])

						// Send remaining thinking content
						contentChan <- utils.ContentUpdateMsg{
							PanelName: utils.MessageThinking,
							Content:   parts[0],
							Replace:   false,
						}
					}

					// Send final thinking panel
					contentChan <- utils.ContentUpdateMsg{
						PanelName: utils.MessageThinking,
						Content:   "",
						Replace:   false,
						Finalize:  true,
					}

					inThinkingTag = false

					// Continue with response if there's content after </think>
					if len(parts) > 1 && parts[1] != "" {
						responseContent.WriteString(parts[1])
						contentChan <- utils.ContentUpdateMsg{
							PanelName: utils.MessageResponse,
							Content:   parts[1],
							Replace:   false,
						}
					}
					return nil
				}

				// Still in thinking block - accumulate and send updates
				thinkingContent.WriteString(chunkStr)
				contentChan <- utils.ContentUpdateMsg{
					PanelName: utils.MessageThinking,
					Content:   chunkStr,
					Replace:   false,
				}
				return nil
			}

			// Normal response content (not in thinking block)
			streamBuffer += chunkStr
			responseContent.WriteString(chunkStr)

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
					PanelName: utils.MessageResponse,
					Content:   streamBuffer,
					Replace:   false,
				}
				streamBuffer = "" // Clear buffer
			}

			return nil
		}),
	}
	if len(a.modelOptions) > 0 {
		callOptions = append(callOptions, a.modelOptions...)
	}

	err := a.model.InvokeStream(a.ctx, messages, callOptions...)

	// Flush any remaining content in buffer
	if streamBuffer != "" {
		contentChan <- utils.ContentUpdateMsg{
			PanelName: utils.MessageResponse,
			Content:   streamBuffer,
			Replace:   false,
		}
	}

	// Finalize the response panel
	if responseContent.Len() > 0 {
		contentChan <- utils.ContentUpdateMsg{
			PanelName: utils.MessageResponse,
			Content:   "",
			Replace:   false,
			Finalize:  true,
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

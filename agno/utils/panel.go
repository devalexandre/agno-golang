package utils

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/devalexandre/agno-golang/agno/utils/terminal"
)

// MessageType for dynamic messages
type MessageType string

const (
	MessageThinking MessageType = "Thinking"
	MessageToolCall MessageType = "Tool Call"
	MessageResponse MessageType = "Response"
	MessageError    MessageType = "Error"
	MessageDebug    MessageType = "Debug"
	MessageSystem   MessageType = "System"
	MessageWarning  MessageType = "Warning"
)

// ContentUpdateMsg represents an update to a panel
type ContentUpdateMsg struct {
	PanelName MessageType
	Content   string
}

// Global renderer instance
var globalRenderer *terminal.PanelRenderer

// init initializes the global renderer
func init() {
	width := terminal.GetTerminalWidth()
	globalRenderer = terminal.NewRenderer(width, false)
}

// GetRenderer returns the global renderer instance
func GetRenderer() *terminal.PanelRenderer {
	return globalRenderer
}

// SetMarkdownMode enables or disables markdown rendering
func SetMarkdownMode(enabled bool) {
	width := terminal.GetTerminalWidth()
	globalRenderer = terminal.NewRenderer(width, enabled)
}

// ThinkingPanel displays a thinking panel and returns nil (for compatibility)
func ThinkingPanel(content string) interface{} {
	panel := globalRenderer.RenderThinking(content)
	fmt.Println(panel)
	return nil // Return nil for compatibility with old spinner interface
}

// ResponsePanel displays a response panel with timing
func ResponsePanel(content string, sp interface{}, start time.Time, markdown bool) {
	duration := time.Since(start).Seconds()

	// Update renderer if markdown setting changed
	if markdown != globalRenderer.Markdown {
		width := terminal.GetTerminalWidth()
		globalRenderer = terminal.NewRenderer(width, markdown)
	}

	panel := globalRenderer.RenderResponse(content, duration)
	fmt.Println(panel)
}

// ToolCallPanel displays a tool call panel
func ToolCallPanel(content string) {
	panel := globalRenderer.RenderToolCallSimple(content)
	fmt.Println(panel)
}

// ToolCallPanelWithArgs displays a tool call panel with structured arguments
func ToolCallPanelWithArgs(toolName string, args interface{}) {
	panel := globalRenderer.RenderToolCall(toolName, args)
	fmt.Println(panel)
}

// DebugPanel displays a debug panel
func DebugPanel(content string) {
	panel := globalRenderer.RenderDebug(content)
	fmt.Println(panel)
}

// ErrorPanel displays an error panel
func ErrorPanel(err error) {
	panel := globalRenderer.RenderError(err)
	fmt.Println(panel)
}

// SuccessPanel displays a success panel
func SuccessPanel(content string) {
	panel := globalRenderer.RenderSuccess(content)
	fmt.Println(panel)
}

// WarningPanel displays a warning panel
func WarningPanel(content string) {
	panel := globalRenderer.RenderWarning(content)
	fmt.Println(panel)
}

// InfoPanel displays an info panel
func InfoPanel(content string) {
	panel := globalRenderer.RenderInfo(content)
	fmt.Println(panel)
}

// ReasoningPanel displays a reasoning panel
func ReasoningPanel(content string) {
	panel := globalRenderer.RenderCustom("💭", "Reasoning...", content, terminal.WarningColor)
	fmt.Println(panel)
}

// StartSimplePanel starts a simple streaming panel
// Returns a channel to send content updates
func StartSimplePanel(sp interface{}, start time.Time, markdown bool) chan<- ContentUpdateMsg {
	contentChan := make(chan ContentUpdateMsg, 10)

	// Update renderer if markdown setting changed
	if markdown != globalRenderer.Markdown {
		width := terminal.GetTerminalWidth()
		globalRenderer = terminal.NewRenderer(width, markdown)
	}

	go func() {
		var responseAccumulator string
		var lastHeight int

		for update := range contentChan {
			if update.PanelName == MessageResponse {
				responseAccumulator += update.Content

				// Calculate height of previous print to clear
				if lastHeight > 0 {
					fmt.Printf("\033[%dA", lastHeight) // Move up
					fmt.Print("\033[J")                // Clear below
				}

				// Render new panel
				duration := time.Since(start).Seconds()
				panel := globalRenderer.RenderResponse(responseAccumulator, duration)

				fmt.Print(panel)
				fmt.Println() // Add newline for visual separation

				// Calculate new height (including the extra newline)
				lastHeight = lipgloss.Height(panel) + 1
			} else {
				printPanel(update.PanelName, update.Content, sp, start, markdown)
			}
		}
	}()

	return contentChan
}

// printPanel prints a panel based on its type
func printPanel(panelName MessageType, content string, sp interface{}, start time.Time, markdown bool) {
	switch panelName {
	case MessageError:
		ErrorPanel(fmt.Errorf("%s", content))
	case MessageWarning:
		WarningPanel(content)
	case MessageDebug:
		DebugPanel(content)
	case MessageSystem:
		InfoPanel(content)
	case MessageToolCall:
		ToolCallPanel(content)
	case MessageResponse:
		// For streaming, just print the content directly
		fmt.Print(content)
	case MessageThinking:
		ThinkingPanel(content)
	default:
		// Default to info panel
		fmt.Println(globalRenderer.RenderCustom("ℹ", string(panelName), content, terminal.InfoColor))
	}
}

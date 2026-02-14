package utils

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/utils/terminal"
	"golang.org/x/term"
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
	AgentName string // Name of the agent producing this content
	Color     string // Color code for the panel (e.g., "cyan", "green", "yellow", "magenta")
	Replace   bool   // If true, replaces the current panel accumulator instead of appending
	Finalize  bool   // If true, renders the final version of the panel (e.g., include duration)
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

func ThinkPanel(content string) interface{} {
	panel := globalRenderer.RenderThink(content, 0)
	fmt.Println(panel)
	return nil // Return nil for compatibility with old spinner interface
}

func ThinkPanelWithDuration(content string, duration float64) interface{} {
	panel := globalRenderer.RenderThink(content, duration)
	fmt.Println(panel)
	return nil
}

func MessagePanel(content string) interface{} {
	panel := globalRenderer.RenderMessage(content)
	fmt.Println(panel)
	return nil
}

// PromptPanel is kept for backward compatibility.
func PromptPanel(content string) interface{} {
	return MessagePanel(content)
}

// ResponsePanel displays a response panel with timing
func ResponsePanel(content string, sp interface{}, start time.Time, markdown bool) {
	duration := time.Since(start).Seconds()

	thinking, final := ExtractThink(content)
	if thinking != "" {
		ThinkPanelWithDuration(thinking, duration)
	}
	content = final

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
		var panelAccumulator strings.Builder
		var panelHeight int
		var lastAgentName string
		var activePanel MessageType
		isPanelActive := false

		// Check if stdout is a TTY
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))

		countLines := func(s string) int {
			if s == "" {
				return 0
			}
			lines := strings.Split(s, "\n")
			return len(lines)
		}

		redrawPanel := func(panel string) {
			newHeight := countLines(panel)

			if !isPanelActive {
				fmt.Print(panel)
				panelHeight = newHeight
				isPanelActive = true
				return
			}

			// Only redraw if we're in a TTY, otherwise just print new content
			if !isTTY {
				// Not a TTY - don't try to redraw, just skip
				return
			}

			// Clear the previous panel and redraw
			// We need to move up to the start of the previous panel
			if panelHeight > 1 {
				// Move cursor up to the first line of the previous panel
				// panelHeight includes the final empty line, so we need panelHeight-1
				fmt.Printf("\033[%dA", panelHeight-1)
				// Move cursor to beginning of line
				fmt.Print("\r")
				// Clear from cursor to end of screen
				fmt.Print("\033[J")
			} else {
				// Just clear current line
				fmt.Print("\r\033[K")
			}

			// Print new panel
			fmt.Print(panel)
			panelHeight = newHeight
		}

		finalizeActivePanel := func() {
			if isPanelActive {
				// Move to end of panel if not already there
				fmt.Println()
			}
			panelAccumulator.Reset()
			panelHeight = 0
			activePanel = ""
			isPanelActive = false
		}

		renderStreamingPanel := func(panelName MessageType, content string, duration float64, finalize bool) string {
			switch panelName {
			case MessageThinking:
				if finalize {
					return globalRenderer.RenderThink(content, duration)
				}
				return globalRenderer.RenderThinking(content)
			case MessageResponse:
				return globalRenderer.RenderResponse(content, duration)
			default:
				return content
			}
		}

		for update := range contentChan {
			isStreamingPanel := update.PanelName == MessageResponse || update.PanelName == MessageThinking

			if isStreamingPanel {
				// If switching panel types, finalize the previous streaming panel first.
				if activePanel != "" && activePanel != update.PanelName {
					finalizeActivePanel()
				}
				activePanel = update.PanelName

				// For workflows, we support "agent" headers above each streaming panel.
				if update.AgentName != "" && update.AgentName != lastAgentName {
					if lastAgentName != "" {
						finalizeActivePanel()
					}
					fmt.Printf("\n▼ %s\n", formatAgentHeader(update.AgentName, update.Color))
					lastAgentName = update.AgentName
				}

				if update.Replace {
					panelAccumulator.Reset()
				}
				panelAccumulator.WriteString(update.Content)

				duration := time.Since(start).Seconds()
				panel := renderStreamingPanel(update.PanelName, panelAccumulator.String(), duration, update.Finalize)

				// Only redraw if TTY or if this is the final update
				if isTTY || update.Finalize {
					redrawPanel(panel)
				}
				continue
			}

			// Non-streaming panels: finalize any active streaming panel and print normally.
			if isPanelActive {
				finalizeActivePanel()
			}
			printPanel(update.PanelName, update.Content, sp, start, markdown)
		}
		finalizeActivePanel()
	}()

	return contentChan
}

// formatAgentHeader formats the agent name with color for professional display
func formatAgentHeader(agentName string, color string) string {
	colorCode := getColorCode(color)
	resetCode := "\033[0m"
	return fmt.Sprintf("%s🤖 %s%s", colorCode, agentName, resetCode)
}

// getColorCode returns ANSI color code for the specified color
func getColorCode(color string) string {
	switch color {
	case "cyan":
		return "\033[36m"
	case "blue":
		return "\033[34m"
	case "green":
		return "\033[32m"
	case "yellow":
		return "\033[33m"
	case "magenta":
		return "\033[35m"
	case "red":
		return "\033[31m"
	case "white":
		return "\033[37m"
	default:
		return "\033[37m"
	}
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

func ExtractThink(content string) (thinking string, final string) {
	const (
		startTag = "<think>"
		endTag   = "</think>"
	)

	start := strings.Index(content, startTag)
	end := strings.Index(content, endTag)

	// Caso exista bloco <think>...</think>
	if start != -1 && end != -1 && end > start {
		thinking := strings.TrimSpace(
			content[start+len(startTag) : end],
		)

		// Remove o bloco <think>...</think> do conteúdo final
		final = strings.TrimSpace(
			content[:start] + content[end+len(endTag):],
		)

		return thinking, final
	}

	// Se não houver <think>, retorna tudo normalmente
	return "", strings.TrimSpace(content)
}

// Think removes <think>...</think> from content and prints it using ThinkPanel.
// Prefer ExtractThink + ThinkPanelWithDuration when duration is available.
func Think(content string) string {
	thinking, final := ExtractThink(content)
	if thinking != "" {
		ThinkPanel(thinking)
	}
	return final
}

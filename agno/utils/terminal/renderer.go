package terminal

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// PanelRenderer handles rendering of different panel types
type PanelRenderer struct {
	styles     *PanelStyles
	maxWidth   int
	Markdown   bool // Exported for external access
	glamourRdr *glamour.TermRenderer
}

// NewRenderer creates a new panel renderer with default settings
func NewRenderer(maxWidth int, useMarkdown bool) *PanelRenderer {
	styles := NewPanelStyles()

	// Adjust width if needed (leave room for borders and padding)
	if maxWidth > 0 {
		contentWidth := maxWidth - 12 // Account for borders, padding, margins (safer)
		if contentWidth < 40 {
			contentWidth = 40 // Minimum width
		}
		styles.WithWidth(contentWidth)
	}

	// Initialize glamour for markdown rendering
	var glamourRdr *glamour.TermRenderer
	if useMarkdown {
		width := maxWidth
		if width == 0 {
			width = 80
		}
		glamourRdr, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width-12),
		)
	}

	return &PanelRenderer{
		styles:     styles,
		maxWidth:   maxWidth,
		Markdown:   useMarkdown,
		glamourRdr: glamourRdr,
	}
}

// RenderThinking renders a thinking/processing panel
func (r *PanelRenderer) RenderThinking(content string) string {
	title := TitleStyle("🤔", "Thinking...", ThinkingColor)

	panel := r.styles.Thinking.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderResponse renders a response panel with timing information
func (r *PanelRenderer) RenderResponse(content string, duration float64) string {
	title := TitleStyle("✨", fmt.Sprintf("Response (%.1fs)", duration), ResponseColor)

	// Render markdown if enabled
	if r.Markdown && r.glamourRdr != nil {
		rendered, err := r.glamourRdr.Render(content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}

	panel := r.styles.Response.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderToolCall renders a tool call panel with function name and arguments
func (r *PanelRenderer) RenderToolCall(toolName string, args interface{}) string {
	title := TitleStyle("🔧", "Tool Call", ToolCallColor)

	// Format arguments as JSON
	argsJSON, err := json.MarshalIndent(args, "", "  ")
	argsStr := string(argsJSON)
	if err != nil {
		argsStr = fmt.Sprintf("%v", args)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		lipgloss.NewStyle().Bold(true).Render("Function: ")+toolName,
		"",
		lipgloss.NewStyle().Bold(true).Render("Arguments:"),
		argsStr,
	)

	panel := r.styles.ToolCall.Render(content)
	return panel
}

// RenderToolCallSimple renders a simple tool call message
func (r *PanelRenderer) RenderToolCallSimple(content string) string {
	title := TitleStyle("🔧", "Tool Call", ToolCallColor)

	panel := r.styles.ToolCall.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderDebug renders a debug information panel
func (r *PanelRenderer) RenderDebug(content string) string {
	title := TitleStyle("🐛", "Debug", DebugColor)

	panel := r.styles.Debug.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderError renders an error panel
func (r *PanelRenderer) RenderError(err error) string {
	title := TitleStyle("❌", "Error", ErrorColor)

	panel := r.styles.Error.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			err.Error(),
		),
	)

	return panel
}

// RenderSuccess renders a success panel
func (r *PanelRenderer) RenderSuccess(content string) string {
	title := TitleStyle("✓", "Success", SuccessColor)

	panel := r.styles.Success.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderWarning renders a warning panel
func (r *PanelRenderer) RenderWarning(content string) string {
	title := TitleStyle("⚠", "Warning", WarningColor)

	panel := r.styles.Warning.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderInfo renders an info panel
func (r *PanelRenderer) RenderInfo(content string) string {
	title := TitleStyle("ℹ", "Info", InfoColor)

	panel := r.styles.Info.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			"",
			content,
		),
	)

	return panel
}

// RenderCustom renders a custom panel with specified title, emoji, color and content
func (r *PanelRenderer) RenderCustom(emoji, title, content string, color lipgloss.Color) string {
	titleStyled := TitleStyle(emoji, title, color)

	customStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Foreground(color).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	if r.maxWidth > 0 {
		customStyle = customStyle.Width(r.maxWidth - 8)
	}

	panel := customStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyled,
			"",
			content,
		),
	)

	return panel
}

// RenderMarkdown renders markdown content if markdown mode is enabled
func (r *PanelRenderer) RenderMarkdown(content string) string {
	if !r.Markdown || r.glamourRdr == nil {
		return content
	}
	
	rendered, err := r.glamourRdr.Render(content)
	if err != nil {
		return content
	}
	
	return strings.TrimSpace(rendered)
}

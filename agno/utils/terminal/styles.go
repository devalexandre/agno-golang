package terminal

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette - Modern, vibrant colors
var (
	ThinkingColor = lipgloss.Color("#00D9FF") // Cyan
	MessageColor  = lipgloss.Color("#10B981") // Green
	ResponseColor = lipgloss.Color("#7C3AED") // Purple
	ToolCallColor = lipgloss.Color("#F59E0B") // Amber
	DebugColor    = lipgloss.Color("#6B7280") // Gray
	ErrorColor    = lipgloss.Color("#EF4444") // Red
	SuccessColor  = lipgloss.Color("#10B981") // Green
	WarningColor  = lipgloss.Color("#F59E0B") // Amber
	InfoColor     = lipgloss.Color("#3B82F6") // Blue
)

// PanelStyles contains all the styled components for different panel types
type PanelStyles struct {
	Thinking lipgloss.Style
	Message  lipgloss.Style
	Response lipgloss.Style
	ToolCall lipgloss.Style
	Debug    lipgloss.Style
	Error    lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Info     lipgloss.Style
}

// NewPanelStyles creates a new set of panel styles with default configuration
func NewPanelStyles() *PanelStyles {
	baseStyle := lipgloss.NewStyle().
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	return &PanelStyles{
		Thinking: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ThinkingColor).
			Foreground(ThinkingColor),

		Message: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(MessageColor).
			Foreground(MessageColor),

		Response: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ResponseColor).
			Foreground(lipgloss.Color("#FFFFFF")),

		ToolCall: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ToolCallColor).
			Foreground(ToolCallColor),

		Debug: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DebugColor).
			Foreground(DebugColor),

		Error: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ErrorColor).
			Foreground(ErrorColor),

		Success: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SuccessColor).
			Foreground(SuccessColor),

		Warning: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(WarningColor).
			Foreground(WarningColor),

		Info: baseStyle.Copy().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(InfoColor).
			Foreground(InfoColor),
	}
}

// WithWidth sets the width for all panel styles
func (ps *PanelStyles) WithWidth(width int) *PanelStyles {
	ps.Thinking = ps.Thinking.Width(width)
	ps.Message = ps.Message.Width(width)
	ps.Response = ps.Response.Width(width)
	ps.ToolCall = ps.ToolCall.Width(width)
	ps.Debug = ps.Debug.Width(width)
	ps.Error = ps.Error.Width(width)
	ps.Success = ps.Success.Width(width)
	ps.Warning = ps.Warning.Width(width)
	ps.Info = ps.Info.Width(width)
	return ps
}

// TitleStyle creates a styled title for panels
func TitleStyle(emoji string, text string, color lipgloss.Color) string {
	style := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
	if emoji == "" {
		return style.Render(text)
	}
	return style.Render(emoji + " " + text)
}

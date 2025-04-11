package utils

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Color lipgloss.Color

const (
	ColorCyan    Color = "12"
	ColorMagenta Color = "13"
	ColorRed     Color = "9"
	ColorGreen   Color = "10"
	ColorYellow  Color = "11"
)

// Original fixed panel
func createPanel(content, title string, color Color, totalWidth int) string {
	// External panel occupies 99% of terminal width
	panelWidth := int(float64(totalWidth) * 0.99)

	// Internal content occupies 98% of the external panel
	contentWidth := int(float64(panelWidth) * 0.98)

	// External panel
	style := lipgloss.NewStyle().
		Width(panelWidth).
		Padding(1, 1).
		Align(lipgloss.Left).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(color))

	// Internal content, not centered
	contentStyle := lipgloss.NewStyle().
		Width(contentWidth). // Define desired internal width
		Align(lipgloss.Left) // Align to the left

	return style.Render(
		contentStyle.Render(fmt.Sprintf("%s\n\n%s", title, content)),
	)
}

func printPanel(content, title string, color Color, width int) {
	panel := createPanel(content, title, color, width)
	fmt.Println(panel)
}

func CreateResponsePanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Response (%.1fs)", elapsedSeconds)
	width, _, _ := termSize()
	printPanel(content, title, ColorCyan, width)
}

func CreateErrorPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Error (%.1fs)", elapsedSeconds)
	width, _, _ := termSize()
	printPanel(content, title, ColorRed, width)
}

func CreateThinkingPanel(question string) {
	width, _, _ := termSize()
	printPanel(question, "Thinking...", ColorGreen, width)
}

func CreateDebugPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("DEBUG (%.1fs)", elapsedSeconds)
	width, _, _ := termSize()
	printPanel(content, title, ColorYellow, width)
}

func CreateSystemPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("System (%.1fs)", elapsedSeconds)
	width, _, _ := termSize()
	printPanel(content, title, ColorGreen, width)
}

func CreateToolCallPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Tool Call (%.1fs)", elapsedSeconds)
	width, _, _ := termSize()
	printPanel(content, title, ColorMagenta, width)
}

func CreateWarningPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Warning (%.1fs)", elapsedSeconds)
	width, _, _ := termSize()
	printPanel(content, title, ColorYellow, width)
}

// Support for dual dynamic panel
type ContentUpdateMsg struct {
	TopPanel    string
	BottomPanel string
}

type model struct {
	topPanel    string
	bottomPanel string
	topColor    Color
	bottomColor Color
	width       int
}

func newModel(initialTop, initialBottom string, topColor, bottomColor Color) model {
	width, _, _ := termSize()
	return model{
		topPanel:    initialTop,
		bottomPanel: initialBottom,
		topColor:    topColor,
		bottomColor: bottomColor,
		width:       width,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case ContentUpdateMsg:
		if msg.TopPanel != "" {
			m.topPanel = msg.TopPanel
		}
		if msg.BottomPanel != "" {
			m.bottomPanel = msg.BottomPanel
		}
	}
	return m, nil
}

func (m model) View() string {
	top := createPanel(m.topPanel, "", m.topColor, m.width)
	bottom := createPanel(m.bottomPanel, "", m.bottomColor, m.width)
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func StartDynamicDualPanel(initialTop, initialBottom string, topColor, bottomColor Color) (chan<- ContentUpdateMsg, <-chan struct{}) {
	contentChan := make(chan ContentUpdateMsg)
	done := make(chan struct{})

	m := newModel(initialTop, initialBottom, topColor, bottomColor)
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		defer close(done)
		for update := range contentChan {
			p.Send(update)
		}
		p.Send(tea.Quit())
	}()

	go func() {
		_, _ = p.Run()
	}()

	return contentChan, done
}

// termSize returns the current terminal size
func termSize() (int, int, error) {
	w, h := lipgloss.Size("")
	return w, h, nil
}

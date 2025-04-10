package utils

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Enum de cores
type Color string

const (
	ColorBlue    Color = "blue"
	ColorMagenta Color = "magenta"
	ColorRed     Color = "red"
	ColorGreen   Color = "green"
	ColorYellow  Color = "yellow"
)

// Estilo base do painel
var panelStyle = lipgloss.NewStyle().
	Padding(1, 2).
	Border(lipgloss.NormalBorder()).
	Align(lipgloss.Left).
	Width(80)

// Cria o painel base genérico
func createPanel(content, title string, color Color) string {
	style := panelStyle.Copy().BorderForeground(lipgloss.Color(string(color)))
	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

// PrintPanel imprime diretamente o painel no console
func printPanel(content, title string, color Color) {
	panel := createPanel(content, title, color)
	fmt.Println(panel)
}

// Painel de resposta ✅
func CreateResponsePanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Response (%.1fs)", elapsedSeconds)
	printPanel(content, title, ColorBlue)
}

// Painel de chamada de ferramenta ✅
func CreateToolCallPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Tool Call (%.1fs)", elapsedSeconds)
	printPanel(content, title, ColorMagenta)
}

// Painel de erro ✅
func CreateErrorPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Error (%.1fs)", elapsedSeconds)
	printPanel(content, title, ColorRed)
}

// Painel do sistema (opcional) ✅
func CreateSystemPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("System (%.1fs)", elapsedSeconds)
	printPanel(content, title, ColorGreen)
}

// Painel de aviso (opcional) ✅
func CreateWarningPanel(content string, elapsedSeconds float64) {
	title := fmt.Sprintf("Warning (%.1fs)", elapsedSeconds)
	printPanel(content, title, ColorYellow)
}

// Painel "Thinking..." padrão ✅
func CreateThinkingPanel(question string) {
	printPanel(question, "Thinking...", ColorYellow)
}

package utils

import (
	"fmt"
	"strings"
	"time"

	mkdown "github.com/MichaelMure/go-term-markdown"
	"github.com/pterm/pterm"
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

// StartSimplePanel starts the simple printing loop
func StartSimplePanel(spinner *pterm.SpinnerPrinter, start time.Time, markdown bool) chan<- ContentUpdateMsg {
	contentChan := make(chan ContentUpdateMsg)

	go func() {
		for update := range contentChan {
			printPanel(update.PanelName, update.Content, spinner, start, markdown)
		}
	}()

	return contentChan
}

// thinking
func ThinkingPanel(content string) *pterm.SpinnerPrinter {
	paddedBox := pterm.DefaultBox.
		WithLeftPadding(4).
		WithRightPadding(4).
		WithTopPadding(1).
		WithBottomPadding(1)

	// Set the box title
	title := pterm.LightGreen("Thinking...")
	paddedBox.
		WithTitle(title).
		WithTextStyle(pterm.NewStyle(pterm.FgGreen)).
		Println(content)

	spinnerResponse, _ := pterm.DefaultSpinner.
		WithWriter(paddedBox.Writer).
		Start("Loading...")

	return spinnerResponse

}

// Reasoning Panel Yellow
func ReasoningPanel(content string) {
	// Get terminal width to prevent overflow
	terminalWidth := pterm.GetTerminalWidth()

	// Ensure minimum width to prevent negative calculations
	if terminalWidth < 30 {
		terminalWidth = 80 // Default fallback
	}

	// Use a fixed width that works well for most terminals
	panelWidth := 75
	if terminalWidth < panelWidth {
		panelWidth = terminalWidth - 4
	}

	// Calculate available content width (account for borders and padding)
	maxContentWidth := panelWidth - 4 // "│ " + " │"

	// Process content lines
	lines := strings.Split(content, "\n")
	var processedLines []string

	for _, line := range lines {
		if len(line) == 0 {
			processedLines = append(processedLines, "")
			continue
		}

		// Wrap long lines
		for len(line) > maxContentWidth {
			processedLines = append(processedLines, line[:maxContentWidth])
			line = line[maxContentWidth:]
		}
		if len(line) > 0 {
			processedLines = append(processedLines, line)
		}
	}

	// Create borders with fixed width
	title := " Reasoning... "
	titleLen := len(title)
	remainingWidth := panelWidth - titleLen - 4 // 4 for "┌─" and "─┐"
	if remainingWidth < 0 {
		remainingWidth = 0
	}

	leftPadding := remainingWidth / 2
	rightPadding := remainingWidth - leftPadding

	topBorder := "┌─" + strings.Repeat("─", leftPadding) + title + strings.Repeat("─", rightPadding) + "─┐"
	bottomBorder := "└" + strings.Repeat("─", panelWidth-2) + "┘"

	// Print the panel
	fmt.Printf("\n%s\n", pterm.LightYellow(topBorder))

	// Add empty line for spacing
	emptyPadding := strings.Repeat(" ", maxContentWidth)
	fmt.Printf("%s %s %s\n", pterm.LightYellow("│"), emptyPadding, pterm.LightYellow("│"))

	for _, line := range processedLines {
		fmt.Println(line)
		// Pad line to fit exactly within borders
		padding := maxContentWidth - len(line)
		if padding < 0 {
			padding = 0
			line = line[:maxContentWidth]
		}
		fmt.Printf("%s %s%s %s\n",
			pterm.LightYellow("│"),
			line,
			strings.Repeat(" ", padding),
			pterm.LightYellow("│"))
	}

	// Add empty line for spacing
	fmt.Printf("%s %s %s\n", pterm.LightYellow("│"), emptyPadding, pterm.LightYellow("│"))

	fmt.Printf("%s\n\n", pterm.LightYellow(bottomBorder))
}

// Debug Panel
func DebugPanel(content string) {
	paddedBox := pterm.DefaultBox.
		WithLeftPadding(4).
		WithRightPadding(4).
		WithTopPadding(1).
		WithBottomPadding(1)

	// Set the box title
	title := pterm.LightYellow("Debug...")
	paddedBox.
		WithTitle(title).
		Println(content)
}

// tools Panel
func ToolCallPanel(content string) {
	paddedBox := pterm.DefaultBox.
		WithLeftPadding(4).
		WithRightPadding(4).
		WithTopPadding(1).
		WithBottomPadding(1)

	// Set the box title
	title := pterm.LightCyan("Tool Call...")
	paddedBox.
		WithTitle(title).
		Println(content)
}

// response panel
func ResponsePanel(content string, sp *pterm.SpinnerPrinter, start time.Time, markdown bool) {
	sp.Stop()
	res := pterm.LightBlue(fmt.Sprintf("Response (%.1fs)\n\n", time.Since(start).Seconds()))
	if markdown {
		content = string(mkdown.Render(content, 100, 0))
	}
	res += content

	// Print the final result instead of just updating spinner text
	fmt.Println(res)
}

// printPanel prints a panel using pterm
func printPanel(panelName MessageType, content string, spinnerResponse *pterm.SpinnerPrinter, stime time.Time, markdown bool) {
	paddedBox := pterm.DefaultBox.
		WithLeftPadding(4).
		WithRightPadding(4).
		WithTopPadding(1).
		WithBottomPadding(1)

	switch panelName {
	case MessageError:
		title := pterm.LightRed("Error...")
		paddedBox.
			WithTitle(title).
			WithTextStyle(pterm.NewStyle(pterm.FgRed))
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)

	case MessageWarning:
		title := pterm.LightYellow("Warning...")
		paddedBox.
			WithTitle(title).
			WithTextStyle(pterm.NewStyle(pterm.FgYellow))
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)

	case MessageDebug:
		title := pterm.LightBlue("Debug...")
		paddedBox.
			WithTitle(title).
			WithBoxStyle(pterm.Debug.MessageStyle)
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)

	case MessageSystem:
		title := pterm.LightMagenta("System...")
		paddedBox.
			WithTitle(title).
			WithTextStyle(pterm.NewStyle(pterm.FgMagenta))
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)

	case MessageToolCall:
		title := pterm.LightCyan("Tool Call...")
		paddedBox.
			WithTitle(title).
			WithTextStyle(pterm.NewStyle(pterm.FgCyan))
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)
	case MessageResponse:
		if markdown {
			content = string(mkdown.Render(content, 100, 0))
		}

		spinnerResponse.Stop()
		paddedBox.
			WithTextStyle(pterm.NewStyle(pterm.FgLightBlue))
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)

	}
}

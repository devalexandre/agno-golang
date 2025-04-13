package utils

import (
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
func StartSimplePanel(spinner *pterm.SpinnerPrinter, start time.Time) chan<- ContentUpdateMsg {
	contentChan := make(chan ContentUpdateMsg)

	go func() {
		for update := range contentChan {
			printPanel(update.PanelName, update.Content, spinner, start)
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

	// Define o título da caixa
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

// Debug Panel
func DebugPanel(content string) {
	paddedBox := pterm.DefaultBox.
		WithLeftPadding(4).
		WithRightPadding(4).
		WithTopPadding(1).
		WithBottomPadding(1)

	// Define o título da caixa
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

	// Define o título da caixa
	title := pterm.LightCyan("Tool Call...")
	paddedBox.
		WithTitle(title).
		Println(content)
}

// response panel
func ResponsePanel(content string, sp *pterm.SpinnerPrinter, start time.Time, markdown bool) {

	sp.Stop()
	res := pterm.LightBlue("Response... \n")
	if markdown {
		content = string(mkdown.Render(content, 100, 0))
	}
	res += content
	sp.UpdateText(res)

}

// printPanel prints a panel using pterm
func printPanel(panelName MessageType, content string, spinnerResponse *pterm.SpinnerPrinter, stime time.Time) {
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
		spinnerResponse.Stop()
		paddedBox.
			WithTextStyle(pterm.NewStyle(pterm.FgLightBlue))
		spinnerResponse.WithWriter(paddedBox.Writer)
		spinnerResponse.UpdateText(content)

	}
}

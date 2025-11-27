package terminal

import (
	"os"

	"golang.org/x/term"
)

// GetTerminalWidth returns the current terminal width
// Returns 80 as default if unable to detect
func GetTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width == 0 {
		return 80 // Default width
	}
	return width
}

// GetTerminalSize returns both width and height of the terminal
func GetTerminalSize() (width, height int) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80, 24 // Default size
	}
	return width, height
}

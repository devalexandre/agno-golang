package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pterm/pterm"
)

// EnvironmentCheck represents a single environment check
type EnvironmentCheck struct {
	Name       string
	CheckFunc  func() (bool, string)
	Required   bool
	ErrorMsg   string
	Suggestion string
}

// ValidateEnvironment checks if all required dependencies are available
func ValidateEnvironment() error {
	checks := []EnvironmentCheck{
		{
			Name:     "OPENROUTER_API_KEY",
			Required: true,
			CheckFunc: func() (bool, string) {
				key := os.Getenv("OPENROUTER_API_KEY")
				if key == "" {
					return false, "not set"
				}
				return true, "configured"
			},
			ErrorMsg:   "OPENROUTER_API_KEY not set",
			Suggestion: "Set your API key: export OPENROUTER_API_KEY=your_key",
		},
		{
			Name:     "Git",
			Required: false,
			CheckFunc: func() (bool, string) {
				cmd := exec.Command("git", "--version")
				out, err := cmd.Output()
				if err != nil {
					return false, ""
				}
				return true, string(out[:len(out)-1]) // Remove newline
			},
			Suggestion: "Install git: https://git-scm.com/downloads",
		},
		{
			Name:     "Go",
			Required: false,
			CheckFunc: func() (bool, string) {
				cmd := exec.Command("go", "version")
				out, err := cmd.Output()
				if err != nil {
					return false, ""
				}
				return true, string(out[:len(out)-1]) // Remove newline
			},
			Suggestion: "Install Go: https://golang.org/dl/",
		},
	}

	var warnings []string
	var errors []string

	for _, check := range checks {
		ok, version := check.CheckFunc()
		if !ok {
			if check.Required {
				errors = append(errors, fmt.Sprintf("✗ %s: %s\n  → %s", check.Name, check.ErrorMsg, check.Suggestion))
			} else {
				warnings = append(warnings, fmt.Sprintf("⚠ %s not found\n  → %s", check.Name, check.Suggestion))
			}
		} else if version != "" {
			pterm.Success.Printf("%s: %s\n", check.Name, version)
		}
	}

	if len(warnings) > 0 {
		pterm.Println()
		pterm.Warning.Println("Optional dependencies missing:")
		for _, w := range warnings {
			pterm.Println("  " + w)
		}
	}

	if len(errors) > 0 {
		pterm.Println()
		pterm.Error.Println("Required dependencies missing:")
		for _, e := range errors {
			pterm.Println("  " + e)
		}
		return fmt.Errorf("environment validation failed")
	}

	return nil
}

package skill

import (
	"fmt"
	"strings"
)

// SkillError is the base error type for all skill-related errors.
type SkillError struct {
	Message string
	Cause   error
}

func (e *SkillError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *SkillError) Unwrap() error { return e.Cause }

// SkillParseError is raised when SKILL.md parsing fails.
type SkillParseError struct {
	SkillError
}

// SkillValidationError is raised when skill validation fails.
type SkillValidationError struct {
	SkillError
	Errors []string
}

func (e *SkillValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0]
	}
	return fmt.Sprintf("%d validation errors: %s", len(e.Errors), strings.Join(e.Errors, "; "))
}

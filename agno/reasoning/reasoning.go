// Package reasoning provides functionality for managing reasoning steps.
package reasoning

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

var (
	ErrInvalidInput = errors.New("invalid input")
)

// ParseReasoningSteps parses thinking text into ReasoningSteps.
func ParseReasoningSteps(thinking string) ([]models.ReasoningStep, error) {
	if thinking == "" {
		return nil, fmt.Errorf("%w: empty input", ErrInvalidInput)
	}

	var steps []models.ReasoningStep
	blocks := strings.Split(thinking, "\n## ")

	for i, block := range blocks {
		if block == "" {
			continue
		}
		if i > 0 {
			block = "## " + block
		}

		step, err := ParseReasoningStepFromModel(block)
		if err != nil {
			return nil, fmt.Errorf("error parsing block %d: %w", i, err)
		}
		steps = append(steps, step)
	}

	return steps, nil
}

// ParseReasoningStepFromModel parses a single reasoning step from text.
func ParseReasoningStepFromModel(content string) (models.ReasoningStep, error) {
	step := models.ReasoningStep{
		NextAction: models.Continue,
	}

	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "## "):
			step.Title = strings.TrimPrefix(line, "## ")
		case strings.HasPrefix(line, "Action: "):
			step.Action = strings.TrimPrefix(line, "Action: ")
		case strings.HasPrefix(line, "Result: "):
			step.Result = strings.TrimPrefix(line, "Result: ")
		case strings.HasPrefix(line, "Confidence: "):
			confStr := strings.TrimPrefix(line, "Confidence: ")
			if conf, err := strconv.ParseFloat(confStr, 64); err == nil {
				step.Confidence = conf
			}
		case strings.HasPrefix(line, "Next: "):
			nextAction, err := models.ParseNextAction(strings.TrimSpace(strings.TrimPrefix(line, "Next: ")))
			if err == nil {
				step.NextAction = nextAction
			}
		default:
			if step.Reasoning == "" {
				step.Reasoning = line
			} else {
				step.Reasoning += "\n" + line
			}
		}
	}

	if err := step.Validate(); err != nil {
		return models.ReasoningStep{}, fmt.Errorf("invalid reasoning step: %w", err)
	}

	return step, nil
}

// FormatReasoningStep formats a ReasoningStep as a string.
func FormatReasoningStep(step models.ReasoningStep) string {
	var builder strings.Builder

	if step.Title != "" {
		builder.WriteString(fmt.Sprintf("## %s\n", step.Title))
	}
	if step.Reasoning != "" {
		builder.WriteString(step.Reasoning + "\n")
	}
	if step.Action != "" {
		builder.WriteString(fmt.Sprintf("Action: %s\n", step.Action))
	}
	if step.Result != "" {
		builder.WriteString(fmt.Sprintf("Result: %s\n", step.Result))
	}
	if step.Confidence > 0 {
		builder.WriteString(fmt.Sprintf("Confidence: %.2f\n", step.Confidence))
	}
	if step.NextAction != "" {
		builder.WriteString(fmt.Sprintf("Next: %s\n", step.NextAction))
	}

	return builder.String()
}

// ReasoningChain executes step-by-step reasoning using the provided model.
func ReasoningChain(
	ctx context.Context,
	modelInvoker func(context.Context, []string) (string, error),
	prompt string,
	minSteps, maxSteps int,
) ([]models.ReasoningStep, error) {
	var steps []models.ReasoningStep

	for i := 0; i < maxSteps; i++ {
		response, err := modelInvoker(ctx, []string{prompt})
		if err != nil {
			return nil, fmt.Errorf("model invocation failed: %w", err)
		}

		step, err := ParseReasoningStepFromModel(response)
		if err != nil {
			return nil, fmt.Errorf("failed to parse reasoning step: %w", err)
		}

		steps = append(steps, step)

		if step.NextAction == models.FinalAnswer && i+1 >= minSteps {
			break
		}

		prompt = response
	}

	return steps, nil
}

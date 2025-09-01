package reasoning_test

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func TestParseReasoningStep(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    models.ReasoningStep
		wantErr bool
	}{
		{
			name: "full step",
			input: `## Step 1
This is the reasoning
Action: Do something
Result: Did something
Confidence: 0.8
Next: continue`,
			want: models.ReasoningStep{
				Title:      "Step 1",
				Reasoning:  "This is the reasoning",
				Action:     "Do something",
				Result:     "Did something",
				Confidence: 0.8,
				NextAction: models.Continue,
			},
		},
		{
			name: "minimal step",
			input: `## Step 2
Just thinking`,
			want: models.ReasoningStep{
				Title:      "Step 2",
				Reasoning:  "Just thinking",
				NextAction: models.Continue,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reasoning.ParseReasoningStepFromModel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %v, want %v", got.Title, tt.want.Title)
			}
			if got.Reasoning != tt.want.Reasoning {
				t.Errorf("Reasoning = %v, want %v", got.Reasoning, tt.want.Reasoning)
			}
			if got.Action != tt.want.Action {
				t.Errorf("Action = %v, want %v", got.Action, tt.want.Action)
			}
		})
	}
}

func TestParseReasoningSteps(t *testing.T) {
	input := `## Step 1
First thought
Action: Do A

## Step 2
Second thought
Action: Do B`

	steps, err := reasoning.ParseReasoningSteps(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	if steps[0].Title != "Step 1" {
		t.Errorf("steps[0].Title = %v, want Step 1", steps[0].Title)
	}
	if steps[1].Title != "Step 2" {
		t.Errorf("steps[1].Title = %v, want Step 2", steps[1].Title)
	}
}

func TestFormatReasoningStep(t *testing.T) {
	step := models.ReasoningStep{
		Title:      "Test Step",
		Reasoning:  "This is a test",
		Action:     "Test action",
		Confidence: 0.9,
		NextAction: models.Continue,
	}

	formatted := reasoning.FormatReasoningStep(step)
	expected := "## Test Step\nThis is a test\nAction: Test action\nConfidence: 0.90\nNext: continue\n"
	
	if formatted != expected {
		t.Errorf("FormatReasoningStep() = %q, want %q", formatted, expected)
	}
}

func TestReasoningChain(t *testing.T) {
	mockInvoker := func(ctx context.Context, prompts []string) (string, error) {
		return "## Test Step\nTest reasoning\nNext: final_answer", nil
	}

	steps, err := reasoning.ReasoningChain(context.Background(), mockInvoker, "test prompt", 1, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Title != "Test Step" {
		t.Errorf("step title = %v, want Test Step", steps[0].Title)
	}
}
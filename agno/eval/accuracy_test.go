package eval

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAccuracyEvalRun(t *testing.T) {
	agent := &mockAgent{outputs: []string{"The capital of France is Paris."}}
	judge := &mockJudge{responses: []string{
		`{"accuracy_score": 9, "accuracy_reason": "Matches the expected output."}`,
		`{"accuracy_score": 7, "accuracy_reason": "Slightly less complete."}`,
		`{"accuracy_score": 8, "accuracy_reason": "Accurate and complete."}`,
	}}

	evaluation := &AccuracyEval{
		Name:           "capital-of-france",
		Agent:          agent,
		Judge:          judge,
		Input:          "What is the capital of France?",
		ExpectedOutput: "Paris",
		NumIterations:  3,
	}

	result, err := evaluation.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 3 {
		t.Fatalf("expected 3 evaluations, got %d", len(result.Results))
	}
	if agent.calls != 3 || judge.calls != 3 {
		t.Fatalf("expected 3 agent and judge calls, got %d and %d", agent.calls, judge.calls)
	}
	if result.AvgScore != 8 {
		t.Fatalf("expected an average score of 8, got %f", result.AvgScore)
	}
	if result.MinScore != 7 || result.MaxScore != 9 {
		t.Fatalf("expected min 7 and max 9, got %f and %f", result.MinScore, result.MaxScore)
	}
	if result.StdDevScore <= 0 {
		t.Fatalf("expected a positive standard deviation, got %f", result.StdDevScore)
	}
	if result.MeanScore != result.AvgScore {
		t.Fatal("the mean score must match the average score")
	}

	// The judge must receive the input, the expected output and the agent output.
	for _, expected := range []string{"<agent_input>", "<expected_output>", "<agent_output>", "Paris"} {
		if !strings.Contains(judge.lastInput, expected) {
			t.Fatalf("the judge input is missing %q: %s", expected, judge.lastInput)
		}
	}

	if evaluation.Result == nil {
		t.Fatal("expected the result to be stored on the evaluation")
	}
	if _, err := result.JSON(); err != nil {
		t.Fatalf("failed to serialize the result: %v", err)
	}
}

func TestAccuracyEvalRunWithOutput(t *testing.T) {
	judge := &mockJudge{responses: []string{
		"```json\n{\"accuracy_score\": 10, \"accuracy_reason\": \"Exact match.\"}\n```",
	}}

	evaluation := &AccuracyEval{
		Judge:                judge,
		Input:                "What is 2 + 2?",
		ExpectedOutput:       "4",
		AdditionalGuidelines: []string{"Only the final number matters."},
		AdditionalContext:    "The user is a student.",
	}

	result, err := evaluation.RunWithOutput(context.Background(), "4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 1 || result.Results[0].Score != 10 {
		t.Fatalf("unexpected result: %+v", result.Results)
	}

	description := evaluation.judgeDescription()
	if !strings.Contains(description, "Only the final number matters.") {
		t.Fatal("the additional guidelines are missing from the judge description")
	}
	if !strings.Contains(description, "The user is a student.") {
		t.Fatal("the additional context is missing from the judge description")
	}
}

func TestAccuracyEvalValidation(t *testing.T) {
	judge := &mockJudge{responses: []string{`{"accuracy_score": 5, "accuracy_reason": "ok"}`}}

	cases := map[string]*AccuracyEval{
		"missing agent": {Judge: judge, Input: "in", ExpectedOutput: "out"},
		"missing judge": {Agent: &mockAgent{outputs: []string{"out"}}, Input: "in", ExpectedOutput: "out"},
		"missing input": {Agent: &mockAgent{outputs: []string{"out"}}, Judge: judge, ExpectedOutput: "out"},
		"missing expected output": {
			Agent: &mockAgent{outputs: []string{"out"}}, Judge: judge, Input: "in",
		},
	}

	for name, evaluation := range cases {
		if _, err := evaluation.Run(context.Background()); err == nil {
			t.Fatalf("expected an error for the case %q", name)
		}
	}
}

func TestAccuracyEvalJudgeFailure(t *testing.T) {
	evaluation := &AccuracyEval{
		Agent:          &mockAgent{outputs: []string{"Paris"}},
		Judge:          &mockJudge{err: errors.New("model unavailable")},
		Input:          "What is the capital of France?",
		ExpectedOutput: "Paris",
	}

	if _, err := evaluation.Run(context.Background()); err == nil {
		t.Fatal("expected an error when the judge fails")
	}
}

func TestAccuracyEvalInvalidJudgeResponse(t *testing.T) {
	evaluation := &AccuracyEval{
		Agent:          &mockAgent{outputs: []string{"Paris"}},
		Judge:          &mockJudge{responses: []string{"I cannot grade this."}},
		Input:          "What is the capital of France?",
		ExpectedOutput: "Paris",
	}

	if _, err := evaluation.Run(context.Background()); err == nil {
		t.Fatal("expected an error when the judge response cannot be parsed")
	}
}

func TestAccuracyResultPrint(t *testing.T) {
	result := &AccuracyResult{Results: []AccuracyEvaluation{
		{Input: "in", Output: "out", ExpectedOutput: "out", Score: 9, Reason: "close enough"},
	}}
	result.ComputeStats()

	var summary, results bytes.Buffer
	result.PrintSummary(&summary)
	result.PrintResults(&results)

	if !strings.Contains(summary.String(), "Average Score") {
		t.Fatalf("unexpected summary: %s", summary.String())
	}
	if !strings.Contains(results.String(), "9/10") {
		t.Fatalf("unexpected results: %s", results.String())
	}
}

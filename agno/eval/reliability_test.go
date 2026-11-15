package eval

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
)

// runResponseWithToolCalls builds a run response containing the given tool calls.
func runResponseWithToolCalls(calls ...tools.ToolCall) *models.RunResponse {
	return &models.RunResponse{
		TextContent: "done",
		Messages: []models.Message{
			{Role: models.TypeUserRole, Content: "what is 10 * 5?"},
			{Role: models.TypeAssistantRole, ToolCalls: calls},
		},
	}
}

// toolCallFor builds a tool call with the given name and JSON arguments.
func toolCallFor(name, arguments string) tools.ToolCall {
	return tools.ToolCall{
		ID:       name + "-1",
		Function: tools.FunctionCall{Name: name, Arguments: arguments},
	}
}

func TestReliabilityEvalPassed(t *testing.T) {
	evaluation := &ReliabilityEval{
		Name:              "math-tools",
		AgentResponse:     runResponseWithToolCalls(toolCallFor("multiply", `{"a": 10, "b": 5}`)),
		ExpectedToolCalls: []string{"multiply"},
	}

	result, err := evaluation.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed() {
		t.Fatalf("expected the evaluation to pass, got %+v", result)
	}
	if err := result.AssertPassed(); err != nil {
		t.Fatalf("unexpected assertion error: %v", err)
	}
	if len(result.PassedToolCalls) != 1 || result.PassedToolCalls[0] != "multiply" {
		t.Fatalf("unexpected passed tool calls: %v", result.PassedToolCalls)
	}
	if _, err := result.JSON(); err != nil {
		t.Fatalf("failed to serialize the result: %v", err)
	}
}

func TestReliabilityEvalMissingToolCall(t *testing.T) {
	evaluation := &ReliabilityEval{
		AgentResponse:     runResponseWithToolCalls(),
		ExpectedToolCalls: []string{"multiply"},
	}

	result, err := evaluation.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EvalStatus != StatusFailed {
		t.Fatal("expected the evaluation to fail")
	}
	if len(result.MissingToolCalls) != 1 || result.MissingToolCalls[0] != "multiply" {
		t.Fatalf("unexpected missing tool calls: %v", result.MissingToolCalls)
	}
	if err := result.AssertPassed(); err == nil {
		t.Fatal("expected an assertion error")
	}
}

func TestReliabilityEvalUnexpectedToolCall(t *testing.T) {
	response := runResponseWithToolCalls(
		toolCallFor("multiply", `{"a": 10, "b": 5}`),
		toolCallFor("search", `{"query": "math"}`),
	)

	failing := &ReliabilityEval{AgentResponse: response, ExpectedToolCalls: []string{"multiply"}}
	result, err := failing.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EvalStatus != StatusFailed {
		t.Fatal("expected the evaluation to fail on an unexpected tool call")
	}
	if len(result.FailedToolCalls) != 1 || result.FailedToolCalls[0] != "search" {
		t.Fatalf("unexpected failed tool calls: %v", result.FailedToolCalls)
	}

	tolerant := &ReliabilityEval{
		AgentResponse:            response,
		ExpectedToolCalls:        []string{"multiply"},
		AllowAdditionalToolCalls: true,
	}
	tolerantResult, err := tolerant.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tolerantResult.Passed() {
		t.Fatalf("expected the evaluation to pass with additional tool calls allowed, got %+v", tolerantResult)
	}
	if len(tolerantResult.AdditionalToolCalls) != 1 || tolerantResult.AdditionalToolCalls[0] != "search" {
		t.Fatalf("unexpected additional tool calls: %v", tolerantResult.AdditionalToolCalls)
	}
}

func TestReliabilityEvalArgumentChecks(t *testing.T) {
	response := runResponseWithToolCalls(
		toolCallFor("add", `{"a": 2, "b": 2}`),
		toolCallFor("add", `{"a": 3, "b": 3}`),
	)

	passing := &ReliabilityEval{
		AgentResponse:     response,
		ExpectedToolCalls: []string{"add"},
		ExpectedToolCallArguments: map[string][]map[string]interface{}{
			"add": {
				{"a": 2, "b": 2},
				{"a": 3},
			},
		},
	}
	result, err := passing.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed() {
		t.Fatalf("expected the argument checks to pass, got %+v", result)
	}
	if len(result.PassedArgumentChecks) != 1 {
		t.Fatalf("unexpected passed argument checks: %v", result.PassedArgumentChecks)
	}

	failing := &ReliabilityEval{
		AgentResponse:     response,
		ExpectedToolCalls: []string{"add"},
		ExpectedToolCallArguments: map[string][]map[string]interface{}{
			"add": {{"a": 9, "b": 9}},
		},
	}
	failingResult, err := failing.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failingResult.EvalStatus != StatusFailed {
		t.Fatal("expected the argument checks to fail")
	}
	if len(failingResult.FailedArgumentChecks) != 1 || failingResult.FailedArgumentChecks[0] != "add" {
		t.Fatalf("unexpected failed argument checks: %v", failingResult.FailedArgumentChecks)
	}
}

func TestReliabilityEvalRequiresResponse(t *testing.T) {
	if _, err := (&ReliabilityEval{ExpectedToolCalls: []string{"multiply"}}).Run(); err == nil {
		t.Fatal("expected an error when no agent response is given")
	}
}

func TestReliabilityResultPrint(t *testing.T) {
	evaluation := &ReliabilityEval{
		AgentResponse:     runResponseWithToolCalls(toolCallFor("multiply", `{"a": 1}`)),
		ExpectedToolCalls: []string{"multiply"},
	}
	result, err := evaluation.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output bytes.Buffer
	result.PrintResults(&output)

	if !strings.Contains(output.String(), "PASSED") {
		t.Fatalf("unexpected output: %s", output.String())
	}
}

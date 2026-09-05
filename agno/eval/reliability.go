package eval

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
)

// ReliabilityResult reports which of the expected tool calls actually happened during a run.
type ReliabilityResult struct {
	// EvalStatus is StatusPassed when every expectation was met.
	EvalStatus Status `json:"eval_status"`
	// PassedToolCalls are the tool calls that were expected and happened.
	PassedToolCalls []string `json:"passed_tool_calls"`
	// FailedToolCalls are the tool calls that happened but were not expected.
	FailedToolCalls []string `json:"failed_tool_calls"`
	// AdditionalToolCalls are unexpected tool calls tolerated by AllowAdditionalToolCalls.
	AdditionalToolCalls []string `json:"additional_tool_calls"`
	// MissingToolCalls are the expected tool calls that never happened.
	MissingToolCalls []string `json:"missing_tool_calls"`
	// PassedArgumentChecks are the tools whose expected arguments were all matched.
	PassedArgumentChecks []string `json:"passed_argument_checks"`
	// FailedArgumentChecks are the tools whose expected arguments were not matched.
	FailedArgumentChecks []string `json:"failed_argument_checks"`
}

// Passed reports whether the evaluation succeeded.
func (r *ReliabilityResult) Passed() bool {
	return r.EvalStatus == StatusPassed
}

// AssertPassed returns an error describing the failure when the evaluation did not pass.
func (r *ReliabilityResult) AssertPassed() error {
	if r.Passed() {
		return nil
	}
	return fmt.Errorf(
		"eval: reliability evaluation failed (missing: %v, unexpected: %v, failed argument checks: %v)",
		r.MissingToolCalls, r.FailedToolCalls, r.FailedArgumentChecks,
	)
}

// JSON serializes the result.
func (r *ReliabilityResult) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// PrintResults writes the result to w. When w is nil, os.Stdout is used.
func (r *ReliabilityResult) PrintResults(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "=== Reliability Evaluation Summary ===")
	fmt.Fprintf(w, "Evaluation Status      : %s\n", r.EvalStatus)
	fmt.Fprintf(w, "Passed Tool Calls      : %v\n", r.PassedToolCalls)
	fmt.Fprintf(w, "Failed Tool Calls      : %v\n", r.FailedToolCalls)
	if len(r.AdditionalToolCalls) > 0 {
		fmt.Fprintf(w, "Additional Tool Calls  : %v\n", r.AdditionalToolCalls)
	}
	if len(r.MissingToolCalls) > 0 {
		fmt.Fprintf(w, "Missing Tool Calls     : %v\n", r.MissingToolCalls)
	}
	if len(r.PassedArgumentChecks) > 0 {
		fmt.Fprintf(w, "Passed Argument Checks : %v\n", r.PassedArgumentChecks)
	}
	if len(r.FailedArgumentChecks) > 0 {
		fmt.Fprintf(w, "Failed Argument Checks : %v\n", r.FailedArgumentChecks)
	}
}

// ReliabilityEval checks that the tool calls expected from a run actually happened.
type ReliabilityEval struct {
	// Name of the evaluation.
	Name string
	// EvalID is a unique identifier for the evaluation. Generated when empty.
	EvalID string

	// AgentResponse is the run being evaluated.
	AgentResponse *models.RunResponse

	// ExpectedToolCalls are the tool names the run is expected to have called.
	ExpectedToolCalls []string
	// AllowAdditionalToolCalls tolerates tool calls that are not in ExpectedToolCalls.
	AllowAdditionalToolCalls bool
	// ExpectedToolCallArguments maps a tool name to the argument sets it must have been called with.
	// Each entry is matched partially: every declared key must be present with the same value in
	// at least one of the actual calls to that tool.
	ExpectedToolCallArguments map[string][]map[string]interface{}

	// PrintResults prints the result after the run.
	PrintResults bool
	// Writer used when printing. Defaults to os.Stdout.
	Writer io.Writer
	// FilePathToSaveResults persists the result as JSON when set.
	FilePathToSaveResults string

	// Result of the last run.
	Result *ReliabilityResult
}

// evalID returns the evaluation ID, generating one when needed.
func (e *ReliabilityEval) evalID() string {
	if e.EvalID == "" {
		e.EvalID = uuid.NewString()
	}
	return e.EvalID
}

// Run evaluates the tool calls of the given run response.
func (e *ReliabilityEval) Run() (*ReliabilityResult, error) {
	if e.AgentResponse == nil {
		return nil, fmt.Errorf("eval: an agent response is required to run the reliability evaluation")
	}

	result := e.evaluate()
	e.Result = result

	if e.FilePathToSaveResults != "" {
		if err := storeResultInFile(e.FilePathToSaveResults, e.Name, e.evalID(), TypeReliability, result); err != nil {
			return nil, err
		}
	}

	if e.PrintResults {
		result.PrintResults(e.Writer)
	}

	return result, nil
}

// evaluate compares the actual tool calls of the run against the expectations.
func (e *ReliabilityEval) evaluate() *ReliabilityResult {
	actualCalls := collectToolCalls(e.AgentResponse)

	result := &ReliabilityResult{
		PassedToolCalls:      []string{},
		FailedToolCalls:      []string{},
		AdditionalToolCalls:  []string{},
		MissingToolCalls:     []string{},
		PassedArgumentChecks: []string{},
		FailedArgumentChecks: []string{},
	}

	calledNames := make(map[string]bool, len(actualCalls))
	for _, call := range actualCalls {
		name := call.Function.Name
		if name == "" {
			continue
		}
		calledNames[name] = true

		switch {
		case contains(e.ExpectedToolCalls, name):
			result.PassedToolCalls = append(result.PassedToolCalls, name)
		case e.AllowAdditionalToolCalls:
			result.AdditionalToolCalls = append(result.AdditionalToolCalls, name)
		default:
			result.FailedToolCalls = append(result.FailedToolCalls, name)
		}
	}

	for _, expected := range e.ExpectedToolCalls {
		if !calledNames[expected] {
			result.MissingToolCalls = append(result.MissingToolCalls, expected)
		}
	}

	for toolName, expectedArgs := range e.ExpectedToolCallArguments {
		if contains(result.MissingToolCalls, toolName) {
			continue
		}
		if e.argumentsMatch(actualCalls, toolName, expectedArgs) {
			result.PassedArgumentChecks = append(result.PassedArgumentChecks, toolName)
		} else {
			result.FailedArgumentChecks = append(result.FailedArgumentChecks, toolName)
		}
	}

	passed := len(result.FailedToolCalls) == 0 &&
		len(result.MissingToolCalls) == 0 &&
		len(result.FailedArgumentChecks) == 0

	result.EvalStatus = StatusFailed
	if passed {
		result.EvalStatus = StatusPassed
	}

	return result
}

// argumentsMatch reports whether every expected argument set was matched by a call to toolName.
func (e *ReliabilityEval) argumentsMatch(actualCalls []tools.ToolCall, toolName string, expectedArgs []map[string]interface{}) bool {
	parsedCalls := make([]map[string]interface{}, 0)
	for _, call := range actualCalls {
		if call.Function.Name != toolName {
			continue
		}
		parsedCalls = append(parsedCalls, parseToolArguments(call.Function.Arguments))
	}
	if len(parsedCalls) == 0 {
		return false
	}

	for _, expected := range expectedArgs {
		matched := false
		for _, actual := range parsedCalls {
			if isPartialMatch(expected, actual) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// collectToolCalls gathers every tool call found in the messages of the given run response.
func collectToolCalls(response *models.RunResponse) []tools.ToolCall {
	calls := make([]tools.ToolCall, 0)
	for _, message := range response.Messages {
		calls = append(calls, message.ToolCalls...)
	}
	return calls
}

// parseToolArguments decodes the JSON arguments of a tool call, returning an empty map on failure.
func parseToolArguments(arguments string) map[string]interface{} {
	parsed := map[string]interface{}{}
	if arguments == "" {
		return parsed
	}
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return map[string]interface{}{}
	}
	return parsed
}

// isPartialMatch reports whether every key of expected is present in actual with an equal value.
func isPartialMatch(expected, actual map[string]interface{}) bool {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok {
			return false
		}
		if !valuesEqual(expectedValue, actualValue) {
			return false
		}
	}
	return true
}

// valuesEqual compares two argument values, normalizing them through JSON so that
// numbers decoded from a tool call match the numeric literals declared in the expectation.
func valuesEqual(expected, actual interface{}) bool {
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	return string(expectedJSON) == string(actualJSON)
}

// contains reports whether the given value is present in the slice.
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

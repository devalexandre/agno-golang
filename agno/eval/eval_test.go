package eval

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

// mockJudge is an offline model used as judge in the tests.
type mockJudge struct {
	responses []string
	err       error
	calls     int
	lastInput string
}

func (m *mockJudge) Invoke(ctx context.Context, messages []models.Message, options ...models.Option) (*models.MessageResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(messages) > 0 {
		m.lastInput = messages[len(messages)-1].Content
	}
	content := m.responses[m.calls%len(m.responses)]
	m.calls++
	return &models.MessageResponse{Role: models.TypeAssistantRole, Content: content}, nil
}

// mockAgent is an offline agent used in the tests.
type mockAgent struct {
	outputs []string
	err     error
	calls   int
	inputs  []interface{}
}

func (m *mockAgent) Run(input interface{}, opts ...interface{}) (models.RunResponse, error) {
	if m.err != nil {
		return models.RunResponse{}, m.err
	}
	m.inputs = append(m.inputs, input)
	output := m.outputs[m.calls%len(m.outputs)]
	m.calls++
	return models.RunResponse{TextContent: output}, nil
}

func TestComputeStats(t *testing.T) {
	stats := computeStats([]float64{2, 4, 4, 4, 5, 5, 7, 9})

	if stats.Count != 8 {
		t.Fatalf("expected count 8, got %d", stats.Count)
	}
	if stats.Avg != 5 {
		t.Fatalf("expected avg 5, got %f", stats.Avg)
	}
	if stats.Min != 2 || stats.Max != 9 {
		t.Fatalf("expected min 2 and max 9, got %f and %f", stats.Min, stats.Max)
	}
	if stats.Median != 4.5 {
		t.Fatalf("expected median 4.5, got %f", stats.Median)
	}
	// Sample standard deviation of the series above.
	if math.Abs(stats.StdDev-2.13809) > 0.0001 {
		t.Fatalf("unexpected std dev: %f", stats.StdDev)
	}
	if stats.P95 < stats.Max-2 || stats.P95 > stats.Max {
		t.Fatalf("p95 out of the expected range: %f", stats.P95)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	stats := computeStats(nil)
	if stats != (Stats{}) {
		t.Fatalf("expected zero stats, got %+v", stats)
	}
}

func TestComputeStatsSingleValue(t *testing.T) {
	stats := computeStats([]float64{3})
	if stats.Avg != 3 || stats.Min != 3 || stats.Max != 3 || stats.Median != 3 || stats.P95 != 3 {
		t.Fatalf("unexpected stats for a single value: %+v", stats)
	}
	if stats.StdDev != 0 {
		t.Fatalf("expected zero std dev, got %f", stats.StdDev)
	}
}

func TestExtractJSONObject(t *testing.T) {
	cases := map[string]string{
		"{\"a\":1}":                      "{\"a\":1}",
		"```json\n{\"a\":1}\n```":        "{\"a\":1}",
		"Here is the result: {\"a\":1}.": "{\"a\":1}",
		"no json here":                   "",
	}

	for input, expected := range cases {
		if got := extractJSONObject(input); got != expected {
			t.Fatalf("extractJSONObject(%q) = %q, expected %q", input, got, expected)
		}
	}
}

func TestParseJudgeResponseInvalidScore(t *testing.T) {
	if _, err := parseJudgeResponse(`{"accuracy_score": 42, "accuracy_reason": "out of range"}`); err == nil {
		t.Fatal("expected an error for an out of range score")
	}
}

func TestStoreResultInFile(t *testing.T) {
	path := t.TempDir() + "/nested/result.json"

	result := &ReliabilityResult{EvalStatus: StatusPassed}
	if err := storeResultInFile(path, "test", "eval-1", TypeReliability, result); err != nil {
		t.Fatalf("failed to store the result: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read back the stored result: %v", err)
	}

	stored := map[string]interface{}{}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("the stored result is not valid JSON: %v", err)
	}
	if stored["eval_type"] != string(TypeReliability) {
		t.Fatalf("unexpected eval type: %v", stored["eval_type"])
	}
	if _, ok := stored["result"]; !ok {
		t.Fatal("the stored file does not contain a result")
	}
}

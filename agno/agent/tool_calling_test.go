package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// MockTool para testes
type MockTool struct {
	toolkit.Toolkit
	callCount int
	delay     time.Duration
}

type MockParams struct {
	Value int `json:"value" description:"Valor de teste"`
}

func (mt *MockTool) TestMethod(params MockParams) (int, error) {
	mt.callCount++
	if mt.delay > 0 {
		time.Sleep(mt.delay)
	}
	return params.Value * 2, nil
}

func createMockTool() *MockTool {
	tool := &MockTool{
		Toolkit: toolkit.Toolkit{
			Name:        "mock",
			Description: "Mock tool for testing",
		},
	}
	tool.Register("test_method", "Mock tool for testing", tool, tool.TestMethod, MockParams{})
	return tool
}

func TestExecuteToolCallsParallel(t *testing.T) {
	ctx := context.Background()
	mockTool := createMockTool()

	agent := &Agent{
		ctx:   ctx,
		tools: []toolkit.Tool{mockTool},
	}

	requests := []ToolCallRequest{
		{
			ToolName:   "mock",
			MethodName: "test_method",
			Arguments:  json.RawMessage(`{"value": 5}`),
		},
		{
			ToolName:   "mock",
			MethodName: "test_method",
			Arguments:  json.RawMessage(`{"value": 10}`),
		},
	}

	config := ToolCallConfig{
		MaxParallelCalls:  2,
		RetryAttempts:     0,
		ValidateArguments: false,
	}

	results := agent.ExecuteToolCallsParallel(ctx, requests, config)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.Success {
			t.Errorf("Result %d failed: %v", i, result.Error)
		}
	}
}

func TestRetryWithBackoff(t *testing.T) {
	ctx := context.Background()
	mockTool := createMockTool()

	agent := &Agent{
		ctx:   ctx,
		tools: []toolkit.Tool{mockTool},
	}

	requests := []ToolCallRequest{
		{
			ToolName:   "mock",
			MethodName: "test_method",
			Arguments:  json.RawMessage(`{"value": 5}`),
		},
	}

	config := ToolCallConfig{
		MaxParallelCalls:      1,
		RetryAttempts:         2,
		RetryDelay:            10,
		UseExponentialBackoff: true,
		ValidateArguments:     false,
	}

	start := time.Now()
	results := agent.ExecuteToolCallsParallel(ctx, requests, config)
	duration := time.Since(start)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if !results[0].Success {
		t.Errorf("Result failed: %v", results[0].Error)
	}

	// Verificar que o delay foi aplicado (pelo menos 10ms)
	if duration < 10*time.Millisecond {
		t.Logf("Duration was %v, expected at least 10ms", duration)
	}
}

func TestArgumentValidation(t *testing.T) {
	ctx := context.Background()
	mockTool := createMockTool()

	agent := &Agent{
		ctx:   ctx,
		tools: []toolkit.Tool{mockTool},
	}

	// Requisição com argumentos inválidos
	requests := []ToolCallRequest{
		{
			ToolName:   "mock",
			MethodName: "test_method",
			Arguments:  json.RawMessage(`{"value": "not a number"}`),
		},
	}

	config := ToolCallConfig{
		MaxParallelCalls:  1,
		RetryAttempts:     0,
		ValidateArguments: true,
	}

	results := agent.ExecuteToolCallsParallel(ctx, requests, config)

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Success {
		t.Error("Expected validation to fail, but it succeeded")
	}

	if results[0].Error == nil {
		t.Error("Expected error for invalid arguments")
	}
}

func TestToolCallStats(t *testing.T) {
	results := []ToolCallResult{
		{
			ToolName:   "mock",
			MethodName: "test1",
			Success:    true,
			Duration:   100 * time.Millisecond,
			Attempt:    1,
		},
		{
			ToolName:   "mock",
			MethodName: "test2",
			Success:    true,
			Duration:   200 * time.Millisecond,
			Attempt:    1,
		},
		{
			ToolName:   "mock",
			MethodName: "test3",
			Success:    false,
			Duration:   50 * time.Millisecond,
			Attempt:    2,
		},
	}

	stats := GetToolCallStats(results)

	if stats["total_calls"] != 3 {
		t.Errorf("Expected 3 total calls, got %d", stats["total_calls"])
	}

	if stats["successful"] != 2 {
		t.Errorf("Expected 2 successful calls, got %d", stats["successful"])
	}

	if stats["failed"] != 1 {
		t.Errorf("Expected 1 failed call, got %d", stats["failed"])
	}

	if stats["total_retries"] != 1 {
		t.Errorf("Expected 1 total retry, got %d", stats["total_retries"])
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	tests := []struct {
		attempt               int
		baseDelayMs           int
		useExponentialBackoff bool
		expectedMin           time.Duration
		expectedMax           time.Duration
	}{
		{
			attempt:               0,
			baseDelayMs:           100,
			useExponentialBackoff: false,
			expectedMin:           85 * time.Millisecond,
			expectedMax:           115 * time.Millisecond,
		},
		{
			attempt:               1,
			baseDelayMs:           100,
			useExponentialBackoff: true,
			expectedMin:           180 * time.Millisecond,
			expectedMax:           220 * time.Millisecond,
		},
		{
			attempt:               2,
			baseDelayMs:           100,
			useExponentialBackoff: true,
			expectedMin:           360 * time.Millisecond,
			expectedMax:           440 * time.Millisecond,
		},
	}

	for _, test := range tests {
		delay := calculateRetryDelay(test.attempt, test.baseDelayMs, test.useExponentialBackoff)

		if delay < test.expectedMin || delay > test.expectedMax {
			t.Errorf("Delay %v not in range [%v, %v]", delay, test.expectedMin, test.expectedMax)
		}
	}
}

func TestValidateArgumentType(t *testing.T) {
	tests := []struct {
		value        interface{}
		expectedType string
		shouldPass   bool
	}{
		{"hello", "string", true},
		{123, "string", false},
		{123.45, "number", true},
		{123, "number", true},
		{true, "boolean", true},
		{false, "boolean", true},
		{[]interface{}{}, "array", true},
		{map[string]interface{}{}, "object", true},
	}

	for _, test := range tests {
		result := validateArgumentType(test.value, test.expectedType)
		if result != test.shouldPass {
			t.Errorf("validateArgumentType(%v, %s) = %v, expected %v", test.value, test.expectedType, result, test.shouldPass)
		}
	}
}

func TestToolCallBatch(t *testing.T) {
	ctx := context.Background()
	mockTool := createMockTool()

	agent := &Agent{
		ctx:   ctx,
		tools: []toolkit.Tool{mockTool},
	}

	batch := &ToolCallBatch{
		ID: "test-batch",
		Requests: []ToolCallRequest{
			{
				ToolName:   "mock",
				MethodName: "test_method",
				Arguments:  json.RawMessage(`{"value": 5}`),
			},
			{
				ToolName:   "mock",
				MethodName: "test_method",
				Arguments:  json.RawMessage(`{"value": 10}`),
			},
		},
		Config: ToolCallConfig{
			MaxParallelCalls:  2,
			RetryAttempts:     0,
			ValidateArguments: false,
		},
	}

	err := agent.ExecuteToolCallBatch(ctx, batch)

	if batch.Status != "completed" {
		t.Errorf("Expected batch status 'completed', got '%s'", batch.Status)
	}

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(batch.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(batch.Results))
	}
}

func TestDefaultToolCallErrorHandler(t *testing.T) {
	handler := NewDefaultToolCallErrorHandler(true)

	successResult := ToolCallResult{
		ToolName:   "mock",
		MethodName: "test",
		Success:    true,
	}

	err := handler.HandleError(successResult)
	if err != nil {
		t.Errorf("Expected no error for successful result, got %v", err)
	}

	failureResult := ToolCallResult{
		ToolName:   "mock",
		MethodName: "test",
		Success:    false,
		Error:      nil,
		Attempt:    1,
	}

	err = handler.HandleError(failureResult)
	if err == nil {
		t.Error("Expected error for failed result")
	}
}

func TestValidateToolCallResponse(t *testing.T) {
	tests := []struct {
		result       interface{}
		expectedType string
		shouldPass   bool
	}{
		{"hello", "string", true},
		{nil, "string", false},
		{123.45, "number", true},
		{true, "boolean", true},
		{[]interface{}{}, "array", true},
		{map[string]interface{}{}, "object", true},
	}

	for _, test := range tests {
		err := ValidateToolCallResponse(test.result, test.expectedType)
		if (err == nil) != test.shouldPass {
			t.Errorf("ValidateToolCallResponse(%v, %s) error = %v, expected pass = %v", test.result, test.expectedType, err, test.shouldPass)
		}
	}
}

func BenchmarkExecuteToolCallsParallel(b *testing.B) {
	ctx := context.Background()
	mockTool := createMockTool()

	agent := &Agent{
		ctx:   ctx,
		tools: []toolkit.Tool{mockTool},
	}

	requests := []ToolCallRequest{
		{
			ToolName:   "mock",
			MethodName: "test_method",
			Arguments:  json.RawMessage(`{"value": 5}`),
		},
	}

	config := ToolCallConfig{
		MaxParallelCalls:  1,
		RetryAttempts:     0,
		ValidateArguments: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.ExecuteToolCallsParallel(ctx, requests, config)
	}
}

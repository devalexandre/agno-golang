package v2

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestBasicWorkflow tests a basic sequential workflow
func TestBasicWorkflow(t *testing.T) {
	// Create a simple workflow with function steps
	step1 := func(input *StepInput) (*StepOutput, error) {
		message := input.GetMessageAsString()
		return &StepOutput{
			Content:  fmt.Sprintf("Step 1: %s", strings.ToUpper(message)),
			StepName: "step1",
		}, nil
	}

	step2 := func(input *StepInput) (*StepOutput, error) {
		previous := fmt.Sprintf("%v", input.PreviousStepContent)
		return &StepOutput{
			Content:  fmt.Sprintf("Step 2 processed: %s", previous),
			StepName: "step2",
		}, nil
	}

	workflow := NewWorkflow(
		WithWorkflowName("Test Workflow"),
		WithWorkflowSteps([]interface{}{step1, step2}),
	)

	ctx := context.Background()
	response, err := workflow.Run(ctx, "hello world")

	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	if response.Status != RunStatusCompleted {
		t.Errorf("Expected status %s, got %s", RunStatusCompleted, response.Status)
	}

	expectedContent := "Step 2 processed: Step 1: HELLO WORLD"
	if response.Content != expectedContent {
		t.Errorf("Expected content '%s', got '%v'", expectedContent, response.Content)
	}
}

// TestConditionalWorkflow tests workflow with conditions
func TestConditionalWorkflow(t *testing.T) {
	condition := NewCondition(
		WithConditionName("test_condition"),
		WithIf(func(input *StepInput) bool {
			message := input.GetMessageAsString()
			return len(message) > 5
		}),
		WithThen(func(input *StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "Long message"}, nil
		}),
		WithElse(func(input *StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "Short message"}, nil
		}),
	)

	workflow := NewWorkflow(
		WithWorkflowName("Conditional Workflow"),
		WithWorkflowSteps([]interface{}{condition}),
	)

	ctx := context.Background()

	// Test with short message
	response1, err := workflow.Run(ctx, "hi")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}
	if response1.Content != "Short message" {
		t.Errorf("Expected 'Short message', got '%v'", response1.Content)
	}

	// Test with long message
	response2, err := workflow.Run(ctx, "hello world")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}
	if response2.Content != "Long message" {
		t.Errorf("Expected 'Long message', got '%v'", response2.Content)
	}
}

// TestParallelWorkflow tests parallel execution
func TestParallelWorkflow(t *testing.T) {
	task1 := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{
			Content:  "Task 1",
			StepName: "task1",
		}, nil
	}

	task2 := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{
			Content:  "Task 2",
			StepName: "task2",
		}, nil
	}

	task3 := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{
			Content:  "Task 3",
			StepName: "task3",
		}, nil
	}

	parallel := NewParallel(
		WithParallelName("test_parallel"),
		WithParallelSteps(task1, task2, task3),
		WithCombineOutputs(true),
	)

	workflow := NewWorkflow(
		WithWorkflowName("Parallel Workflow"),
		WithWorkflowSteps([]interface{}{parallel}),
	)

	ctx := context.Background()
	_, err := workflow.Run(ctx, "start")

	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	// Check that all tasks were executed
	output := workflow.GetStepOutput("test_parallel")
	if output == nil {
		t.Fatal("Parallel step output not found")
	}

	if len(output.ParallelStepOutputs) != 3 {
		t.Errorf("Expected 3 parallel outputs, got %d", len(output.ParallelStepOutputs))
	}

	// Verify each task output
	expectedOutputs := map[string]string{
		"test_parallel_func_0": "Task 1",
		"test_parallel_func_1": "Task 2",
		"test_parallel_func_2": "Task 3",
	}

	for name, expected := range expectedOutputs {
		if output.ParallelStepOutputs[name] == nil {
			t.Errorf("Output for %s not found", name)
			continue
		}
		if output.ParallelStepOutputs[name].Content != expected {
			t.Errorf("Expected %s content '%s', got '%v'", name, expected, output.ParallelStepOutputs[name].Content)
		}
	}
}

// TestLoopWorkflow tests loop execution
func TestLoopWorkflow(t *testing.T) {
	counter := 0
	incrementer := func(input *StepInput) (*StepOutput, error) {
		counter++
		return &StepOutput{
			Content:  fmt.Sprintf("Iteration %d", counter),
			StepName: "incrementer",
		}, nil
	}

	loop := NewLoop(
		WithLoopName("test_loop"),
		WithLoopSteps(incrementer),
		WithMaxIterations(3),
		WithLoopCondition(ForN(3)),
	)

	workflow := NewWorkflow(
		WithWorkflowName("Loop Workflow"),
		WithWorkflowSteps([]interface{}{loop}),
	)

	ctx := context.Background()
	_, err := workflow.Run(ctx, "start")

	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	if counter != 3 {
		t.Errorf("Expected counter to be 3, got %d", counter)
	}

	// Check loop output
	output := workflow.GetStepOutput("test_loop")
	if output == nil {
		t.Fatal("Loop step output not found")
	}

	if len(output.LoopStepOutputs) != 3 {
		t.Errorf("Expected 3 loop outputs, got %d", len(output.LoopStepOutputs))
	}
}

// TestRouterWorkflow tests routing logic
func TestRouterWorkflow(t *testing.T) {
	errorHandler := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{Content: "Error handled"}, nil
	}

	successHandler := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{Content: "Success handled"}, nil
	}

	defaultHandler := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{Content: "Default handled"}, nil
	}

	router := NewRouter(
		WithRouterName("test_router"),
		WithRouteFunc(func(input *StepInput) string {
			message := strings.ToLower(input.GetMessageAsString())
			if strings.Contains(message, "error") {
				return "error"
			} else if strings.Contains(message, "success") {
				return "success"
			}
			return "default"
		}),
		WithRoute("error", errorHandler),
		WithRoute("success", successHandler),
		WithDefaultRoute(defaultHandler),
	)

	workflow := NewWorkflow(
		WithWorkflowName("Router Workflow"),
		WithWorkflowSteps([]interface{}{router}),
	)

	ctx := context.Background()

	// Test error route
	response1, err := workflow.Run(ctx, "This is an error")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}
	if response1.Content != "Error handled" {
		t.Errorf("Expected 'Error handled', got '%v'", response1.Content)
	}

	// Test success route
	response2, err := workflow.Run(ctx, "This is a success")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}
	if response2.Content != "Success handled" {
		t.Errorf("Expected 'Success handled', got '%v'", response2.Content)
	}

	// Test default route
	response3, err := workflow.Run(ctx, "This is normal")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}
	if response3.Content != "Default handled" {
		t.Errorf("Expected 'Default handled', got '%v'", response3.Content)
	}
}

// TestComplexWorkflow tests a workflow with multiple components
func TestComplexWorkflow(t *testing.T) {
	// Step 1: Validate input
	validator := func(input *StepInput) (*StepOutput, error) {
		message := input.GetMessageAsString()
		if message == "" {
			return &StepOutput{
				Content:  "invalid",
				StepName: "validator",
			}, nil
		}
		return &StepOutput{
			Content:  "valid",
			StepName: "validator",
		}, nil
	}

	// Step 2: Process if valid
	processor := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{
			Content:  "Processed successfully",
			StepName: "processor",
		}, nil
	}

	// Step 3: Conditional execution
	condition := NewCondition(
		WithConditionName("validation_check"),
		WithIf(func(input *StepInput) bool {
			return input.PreviousStepContent == "valid"
		}),
		WithThen(processor),
		WithElse(func(input *StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "Skipped processing"}, nil
		}),
	)

	// Step 4: Final step
	finalizer := func(input *StepInput) (*StepOutput, error) {
		allContent := input.GetAllPreviousContent()
		return &StepOutput{
			Content:  fmt.Sprintf("Final: %s", allContent),
			StepName: "finalizer",
		}, nil
	}

	workflow := NewWorkflow(
		WithWorkflowName("Complex Workflow"),
		WithWorkflowSteps([]interface{}{
			validator,
			condition,
			finalizer,
		}),
	)

	ctx := context.Background()

	// Test with valid input
	response1, err := workflow.Run(ctx, "valid input")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	if !strings.Contains(response1.Content.(string), "Processed successfully") {
		t.Errorf("Expected processed output, got '%v'", response1.Content)
	}

	// Test with invalid input
	response2, err := workflow.Run(ctx, "")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	if !strings.Contains(response2.Content.(string), "Skipped processing") {
		t.Errorf("Expected skipped output, got '%v'", response2.Content)
	}
}

// TestWorkflowMetrics tests that metrics are properly collected
func TestWorkflowMetrics(t *testing.T) {
	step1 := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{Content: "Step 1"}, nil
	}

	step2 := func(input *StepInput) (*StepOutput, error) {
		return &StepOutput{Content: "Step 2"}, nil
	}

	workflow := NewWorkflow(
		WithWorkflowName("Metrics Test"),
		WithWorkflowSteps([]interface{}{step1, step2}),
	)

	ctx := context.Background()
	_, err := workflow.Run(ctx, "test")

	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	metrics := workflow.GetMetrics()

	if metrics.StepsExecuted != 2 {
		t.Errorf("Expected 2 steps executed, got %d", metrics.StepsExecuted)
	}

	if metrics.StepsSucceeded != 2 {
		t.Errorf("Expected 2 steps succeeded, got %d", metrics.StepsSucceeded)
	}

	if metrics.StepsFailed != 0 {
		t.Errorf("Expected 0 steps failed, got %d", metrics.StepsFailed)
	}

	if !metrics.Success {
		t.Error("Expected workflow success to be true")
	}

	if metrics.DurationMs < 0 {
		t.Error("Expected non-negative duration")
	}
}

// TestStepInputMethods tests StepInput helper methods
func TestStepInputMethods(t *testing.T) {
	// Create step outputs for testing
	output1 := &StepOutput{
		Content:  "Output 1",
		StepName: "step1",
	}

	output2 := &StepOutput{
		Content:  "Output 2",
		StepName: "step2",
	}

	parallelOutput := &StepOutput{
		StepName: "parallel",
		ParallelStepOutputs: map[string]*StepOutput{
			"task1": {Content: "Task 1"},
			"task2": {Content: "Task 2"},
		},
	}

	input := &StepInput{
		Message:             "test message",
		PreviousStepContent: "previous content",
		PreviousStepOutputs: map[string]*StepOutput{
			"step1":    output1,
			"step2":    output2,
			"parallel": parallelOutput,
		},
	}

	// Test GetMessageAsString
	if input.GetMessageAsString() != "test message" {
		t.Errorf("GetMessageAsString failed")
	}

	// Test GetStepOutput
	if input.GetStepOutput("step1") != output1 {
		t.Errorf("GetStepOutput failed for step1")
	}

	// Test GetStepContent
	if input.GetStepContent("step1") != "Output 1" {
		t.Errorf("GetStepContent failed for step1")
	}

	// Test GetStepContent for parallel step
	parallelContent := input.GetStepContent("parallel")
	if contentMap, ok := parallelContent.(map[string]interface{}); !ok {
		t.Errorf("GetStepContent for parallel step should return a map")
	} else {
		if contentMap["task1"] != "Task 1" {
			t.Errorf("Parallel content for task1 incorrect")
		}
	}

	// Test GetAllPreviousContent
	allContent := input.GetAllPreviousContent()
	if !strings.Contains(allContent, "Output 1") || !strings.Contains(allContent, "Output 2") {
		t.Errorf("GetAllPreviousContent missing expected content")
	}
}

// TestWorkflowEvents tests event handling
func TestWorkflowEvents(t *testing.T) {
	var startedEvents []string
	var completedEvents []string

	workflow := NewWorkflow(
		WithWorkflowName("Event Test"),
		WithWorkflowSteps([]interface{}{
			func(input *StepInput) (*StepOutput, error) {
				return &StepOutput{Content: "Step 1", StepName: "step1"}, nil
			},
		}),
	)

	// Register event handlers
	workflow.OnEvent(StepStartedEvent, func(event *WorkflowRunResponseEvent) {
		if stepName, ok := event.Metadata["step_name"].(string); ok {
			startedEvents = append(startedEvents, stepName)
		}
	})

	workflow.OnEvent(StepCompletedEvent, func(event *WorkflowRunResponseEvent) {
		if stepName, ok := event.Metadata["step_name"].(string); ok {
			completedEvents = append(completedEvents, stepName)
		}
	})

	ctx := context.Background()
	_, err := workflow.Run(ctx, "test")

	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	if len(startedEvents) == 0 {
		t.Error("No step started events received")
	}

	if len(completedEvents) == 0 {
		t.Error("No step completed events received")
	}
}

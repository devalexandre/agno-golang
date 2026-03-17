package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// In-memory storage for demonstration (Durable Workflows usually use Postgres)
type MemoryStorage struct {
	checkpoints map[string]*v2.WorkflowCheckpoint
	mu          sync.RWMutex
}

func (m *MemoryStorage) SaveSession(ctx context.Context, s *v2.WorkflowSession) error { return nil }
func (m *MemoryStorage) LoadSession(ctx context.Context, id string) (*v2.WorkflowSession, error) {
	return nil, nil
}
func (m *MemoryStorage) DeleteSession(ctx context.Context, id string) error { return nil }

func (m *MemoryStorage) SaveCheckpoint(ctx context.Context, sessionID string, checkpoint *v2.WorkflowCheckpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints[sessionID] = checkpoint
	fmt.Printf("[Storage] Saved checkpoint for step index %d\n", checkpoint.NextStepIdx)
	return nil
}

func (m *MemoryStorage) LoadCheckpoint(ctx context.Context, sessionID string) (*v2.WorkflowCheckpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checkpoints[sessionID], nil
}

func main() {
	ctx := context.Background()
	storage := &MemoryStorage{checkpoints: make(map[string]*v2.WorkflowCheckpoint)}
	sessionID := "durable-example-001"

	// Define steps
	step1, _ := v2.NewStep(
		v2.WithName("process-payment"),
		v2.WithExecutor(func(input *v2.StepInput) (*v2.StepOutput, error) {
			return &v2.StepOutput{Content: "Payment Processed"}, nil
		}),
	)

	// This step will fail the first time to simulate an interruption
	failCounter := 0
	step2, _ := v2.NewStep(
		v2.WithName("send-email"),
		v2.WithExecutor(func(input *v2.StepInput) (*v2.StepOutput, error) {
			failCounter++
			if failCounter == 1 {
				return nil, fmt.Errorf("temporary connection failure")
			}
			return &v2.StepOutput{Content: "Email Sent Successfully"}, nil
		}),
	)

	// Create durable workflow
	wf := v2.NewWorkflow(
		v2.WithWorkflowName("Durable Payment Workflow"),
		v2.WithWorkflowSteps([]*v2.Step{step1, step2}),
		v2.WithStorage(storage),
		v2.WithSessionID(sessionID),
		v2.WithDurable(true),
	)

	fmt.Println("--- Running Workflow (First Attempt) ---")
	_, err := wf.Run(ctx, "Start payment")
	if err != nil {
		fmt.Printf("Workflow failed as expected: %v\n", err)
	}

	fmt.Println("\n--- Resuming Workflow (Second Attempt) ---")
	// Step 1 will be skipped because it was already completed and checkpointed
	wf2 := v2.NewWorkflow(
		v2.WithWorkflowName("Durable Payment Workflow"),
		v2.WithWorkflowSteps([]*v2.Step{step1, step2}),
		v2.WithStorage(storage),
		v2.WithSessionID(sessionID),
		v2.WithDurable(true),
	)

	resp, err := wf2.Run(ctx, "Start payment")
	if err != nil {
		fmt.Printf("Workflow failed again: %v\n", err)
	} else {
		fmt.Printf("Workflow Completed Successfully! Result: %s\n", resp.Status)
	}
}

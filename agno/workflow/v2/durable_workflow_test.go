package v2

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// MockStorage implements Workflow Storage for testing durability
type MockStorage struct {
	sessions    map[string]*WorkflowSession
	checkpoints map[string]*WorkflowCheckpoint
	mu          sync.RWMutex
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		sessions:    make(map[string]*WorkflowSession),
		checkpoints: make(map[string]*WorkflowCheckpoint),
	}
}

func (m *MockStorage) SaveSession(ctx context.Context, session *WorkflowSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.SessionID] = session
	return nil
}

func (m *MockStorage) LoadSession(ctx context.Context, sessionID string) (*WorkflowSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

func (m *MockStorage) DeleteSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockStorage) SaveCheckpoint(ctx context.Context, sessionID string, checkpoint *WorkflowCheckpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkpoints[sessionID] = checkpoint
	return nil
}

func (m *MockStorage) LoadCheckpoint(ctx context.Context, sessionID string) (*WorkflowCheckpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	checkpoint, ok := m.checkpoints[sessionID]
	if !ok {
		return nil, nil
	}
	return checkpoint, nil
}

func TestDurableWorkflow(t *testing.T) {
	storage := NewMockStorage()
	sessionID := "test-session"

	// Step 1: Counter that fails on first call, succeeds on second
	callCount := 0
	step1, _ := NewStep(
		WithName("step1"),
		WithMaxRetries(0),
		WithExecutor(func(input *StepInput) (*StepOutput, error) {
			callCount++
			if callCount == 1 {
				return nil, fmt.Errorf("simulated failure in step 1")
			}
			return &StepOutput{Content: "step1 success"}, nil
		}),
	)

	// Step 2: Depends on step 1
	step2, _ := NewStep(
		WithName("step2"),
		WithExecutor(func(input *StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "step2 success"}, nil
		}),
	)

	wf := NewWorkflow(
		WithWorkflowName("Durable Workflow"),
		WithWorkflowSteps([]*Step{step1, step2}),
		WithStorage(storage),
		WithSessionID(sessionID),
		WithDurable(true),
	)

	ctx := context.Background()

	var resp *WorkflowRunResponse
	var err error

	// First run - should fail at step 1
	resp, err = wf.Run(ctx, "start")
	if err == nil {
		t.Errorf("Expected error in first run, got nil")
	}
	if resp != nil && resp.Status != RunStatusFailed {
		t.Errorf("Expected failed status, got %s", resp.Status)
	}

	// Verify no checkpoint saved for step 1 yet (it failed)
	checkpoint, _ := storage.LoadCheckpoint(ctx, sessionID)
	if checkpoint != nil && checkpoint.NextStepIdx > 0 {
		t.Errorf("Expected no checkpoint for step 1 after failure, got idx %d", checkpoint.NextStepIdx)
	}

	// Modify step 1 to succeed now
	// Re-create workflow with same sessionID and storage
	step1Success, _ := NewStep(
		WithName("step1"),
		WithExecutor(func(input *StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "step1 success"}, nil
		}),
	)
	wf2 := NewWorkflow(
		WithWorkflowName("Durable Workflow"),
		WithWorkflowSteps([]*Step{
			step1Success,
			step2,
		}),
		WithStorage(storage),
		WithSessionID(sessionID),
		WithDurable(true),
	)

	// Run again - step 1 should succeed and save checkpoint
	resp, err = wf2.Run(ctx, "start")
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	// Verify checkpoint exists for step 2
	checkpoint, _ = storage.LoadCheckpoint(ctx, sessionID)
	if checkpoint == nil || checkpoint.NextStepIdx != 2 {
		t.Errorf("Expected checkpoint at index 2, got %v", checkpoint)
	}

	// Now simulate failure in step 2
	step2Fail, _ := NewStep(
		WithName("step2"),
		WithMaxRetries(0),
		WithExecutor(func(input *StepInput) (*StepOutput, error) {
			return nil, fmt.Errorf("simulated failure in step 2")
		}),
	)

	step1NeverExecuted, _ := NewStep(
		WithName("step1"),
		WithExecutor(func(input *StepInput) (*StepOutput, error) {
			t.Errorf("Step 1 should not be executed again")
			return nil, fmt.Errorf("step 1 re-executed")
		}),
	)

	wf3 := NewWorkflow(
		WithWorkflowName("Durable Workflow"),
		WithWorkflowSteps([]*Step{
			step1NeverExecuted,
			step2Fail,
		}),
		WithStorage(storage),
		WithSessionID(sessionID),
		WithDurable(true),
	)

	resp, err = wf3.Run(ctx, "start")
	if err == nil {
		t.Errorf("Expected error in third run (step 2), got nil")
	}

	// Verify checkpoint still at index 1 (completed steps)
	checkpoint, _ = storage.LoadCheckpoint(ctx, sessionID)
	if checkpoint == nil || checkpoint.NextStepIdx != 1 {
		t.Errorf("Expected checkpoint at index 1 (step 1 completed), got %v", checkpoint)
	}
}

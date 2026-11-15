// Run status and cancellation: every RunResponse carries Status
// (COMPLETED, ERROR, CANCELLED, ...) and, for a cancelled run, the
// CancellationStage it was in (PENDING = never started, EXECUTING = stopped
// mid-run, PAUSED = parked for human-in-the-loop).
//
// Shows:
//  1. a completed run with a caller-chosen run id (agent.WithRunID);
//  2. a run cancelled while executing with Agent.CancelRun(runID);
//  3. a run cancelled before it started (its context was already cancelled).
//
// Runs offline with an in-file stub model ("slow" prompts block until the
// run's context is cancelled).
//
//	go run ./cookbook/agents/run_status
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
)

func main() {
	ag := newAgent(context.Background())

	// 1. Completed run.
	resp, err := ag.Run("hello", agent.WithRunID("run-ok"))
	if err != nil {
		log.Fatal(err)
	}
	report("completed", resp)

	// 2. Cancel a running run by id from another goroutine.
	go func() {
		time.Sleep(200 * time.Millisecond)
		fmt.Println("CancelRun(run-slow) found the run:", ag.CancelRun("run-slow"))
	}()
	resp, err = ag.Run("slow", agent.WithRunID("run-slow"))
	fmt.Println("error is context.Canceled:", errors.Is(err, context.Canceled))
	report("cancelled while executing", resp)

	// 3. A run whose context is cancelled before it starts never calls the model.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, _ = newAgent(ctx).Run("hello")
	report("cancelled before start", resp)
}

func newAgent(ctx context.Context) *agent.Agent {
	ag, err := agent.NewAgent(agent.AgentConfig{Context: ctx, Model: stubModel{}, Name: "status-demo"})
	if err != nil {
		log.Fatal(err)
	}
	return ag
}

func report(label string, r models.RunResponse) {
	fmt.Printf("[%s] run_id=%q status=%s cancellation_stage=%q cancelled=%v content=%q\n",
		label, r.RunID, r.Status, r.CancellationStage, r.IsCancelled(), r.TextContent)
}

// stubModel echoes the prompt; a "slow" prompt waits for cancellation.
type stubModel struct{}

func (stubModel) GetID() string { return "stub-model" }

func (stubModel) Invoke(ctx context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	last := msgs[len(msgs)-1].Content
	if last == "slow" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return &models.MessageResponse{Role: "assistant", Content: "you said: " + last}, nil
}

func (m stubModel) AInvoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch, errCh := make(chan *models.MessageResponse, 1), make(chan error, 1)
	r, err := m.Invoke(ctx, msgs, opts...)
	if err != nil {
		errCh <- err
	} else {
		ch <- r
	}
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m stubModel) InvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range opts {
		o(call)
	}
	r, err := m.Invoke(ctx, msgs)
	if err != nil {
		return err
	}
	if call.StreamingFunc != nil {
		return call.StreamingFunc(ctx, []byte(r.Content))
	}
	return nil
}

func (m stubModel) AInvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return m.AInvoke(ctx, msgs, opts...)
}

package agent

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/metrics"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/google/uuid"
)

// runState carries per-run bookkeeping that is attached to the RunResponse
// once the run finishes.
type runState struct {
	id         string
	prompt     string
	compaction *models.CompactionRecord
	metrics    *metrics.RunMetrics
}

// applyRunOptions collects run options. It accepts both RunOption values and
// plain func(*RunOptions) values.
func applyRunOptions(opts []interface{}) *RunOptions {
	options := &RunOptions{}
	for _, opt := range opts {
		switch o := opt.(type) {
		case RunOption:
			if o != nil {
				o(options)
			}
		case func(*RunOptions):
			if o != nil {
				o(options)
			}
		}
	}
	return options
}

// WithRunID sets the run ID, so the caller can cancel the run with
// Agent.CancelRun while it executes.
func WithRunID(runID string) RunOption {
	return func(o *RunOptions) {
		o.RunID = &runID
	}
}

// executeRun wraps a run with cancellation tracking and status bookkeeping:
// the response carries RunID, Status (COMPLETED / ERROR / CANCELLED) and, for
// cancelled runs, the CancellationStage at which cancellation hit.
func (a *Agent) executeRun(options *RunOptions, fn func() (models.RunResponse, error)) (models.RunResponse, error) {
	runID := ""
	if options != nil && options.RunID != nil && *options.RunID != "" {
		runID = *options.RunID
	} else {
		runID = uuid.New().String()
	}

	runCtx := a.trackRun(runID)
	prevCtx, prevRun := a.ctx, a.currentRun
	state := &runState{id: runID, metrics: a.newRunMetrics()}
	a.ctx, a.currentRun = runCtx, state
	defer func() {
		a.ctx, a.currentRun = prevCtx, prevRun
		a.untrackRun(runID)
	}()

	finish := func(resp models.RunResponse) models.RunResponse {
		if resp.RunID == "" {
			resp.RunID = runID
		}
		if resp.SessionID == "" {
			resp.SessionID = a.sessionID
		}
		if resp.CreatedAt == 0 {
			resp.CreatedAt = time.Now().Unix()
		}
		if resp.Compaction == nil {
			resp.Compaction = state.compaction
		}
		attachRunMetrics(&resp, state.metrics)
		return resp
	}

	// Cancelled before the run started executing.
	if err := runCtx.Err(); err != nil {
		return finish(models.RunResponse{
			Status:            models.RunStatusCancelled,
			CancellationStage: models.CancellationStagePending,
		}), err
	}

	resp, err := fn()
	resp = finish(resp)
	switch {
	case err == nil && resp.Status == models.RunStatusPaused:
		// Human-in-the-loop pause: the run continues through ContinueRun.
		resp.CancellationStage = ""
	case err == nil:
		resp.Status = models.RunStatusCompleted
		resp.CancellationStage = ""
		a.generateFollowups(runCtx, state, &resp)
	case runCtx.Err() != nil || errors.Is(err, context.Canceled):
		resp.Status = models.RunStatusCancelled
		resp.CancellationStage = models.CancellationStageExecuting
	default:
		resp.Status = models.RunStatusError
	}
	return resp, err
}

// checkCancelled returns the run context's error when the run was cancelled.
func (a *Agent) checkCancelled() error {
	if a.ctx == nil {
		return nil
	}
	return a.ctx.Err()
}

// sleepOrCancel waits for d or until the run is cancelled.
func (a *Agent) sleepOrCancel(d time.Duration) error {
	if a.ctx == nil {
		time.Sleep(d)
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-a.ctx.Done():
		return a.ctx.Err()
	}
}

// generateFollowups fills resp.Followups when follow-ups are enabled. A
// failure is logged and never affects the answer.
func (a *Agent) generateFollowups(ctx context.Context, state *runState, resp *models.RunResponse) {
	resp.Followups = nil
	if a.followups == nil || resp.TextContent == "" {
		return
	}
	suggestions, err := GenerateFollowups(ctx, a.model, a.followups, state.prompt, resp.TextContent)
	if err != nil {
		log.Printf("Warning: error generating followups: %v", err)
		return
	}
	resp.Followups = suggestions
}

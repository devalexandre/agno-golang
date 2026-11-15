// Package jobqueue ports Agno's durable job queue (agno.job_queue and
// agno.os.job_queue): background runs become committed tickets that workers
// claim, heartbeat, complete, or retry-or-fail. Stale claims from crashed
// workers are reclaimed (while attempts remain) or swept to failed. Tickets
// flagged SerializeSession run at most one at a time per session, in
// acceptance order (head-of-line per session).
package jobqueue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Ticket statuses.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusPaused    = "paused"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Enqueue rejection reasons.
const (
	ReasonDuplicate = "duplicate"
	ReasonQueueFull = "queue_full"
)

// Continue outcomes.
const (
	ContinueQueued   = "queued"
	ContinueAttach   = "attach"
	ContinueConflict = "conflict"
)

// ErrNotFound is returned when a job does not exist.
var ErrNotFound = errors.New("jobqueue: job not found")

// Job is a queue ticket.
type Job struct {
	ID            string         `json:"id"`
	JobType       string         `json:"job_type"`
	ComponentType string         `json:"component_type"`
	ComponentID   string         `json:"component_id"`
	SessionID     string         `json:"session_id"`
	UserID        *string        `json:"user_id"`
	DeploymentID  *string        `json:"deployment_id"`
	Payload       map[string]any `json:"payload"`
	// SerializeSession enables head-of-line ordering within SessionID.
	SerializeSession bool    `json:"serialize_session"`
	SessionSeq       int64   `json:"session_seq"`
	Status           string  `json:"status"`
	Attempt          int     `json:"attempt"`
	MaxAttempts      int     `json:"max_attempts"`
	IdempotencyKey   *string `json:"idempotency_key"`
	AvailableAt      int64   `json:"available_at"`
	LockedBy         *string `json:"locked_by"`
	LockedAt         *int64  `json:"locked_at"`
	Error            *string `json:"error"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        *int64  `json:"updated_at"`
	CompletedAt      *int64  `json:"completed_at"`
}

// Clone copies the job (payload shallow-copied).
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	c := *j
	if j.Payload != nil {
		c.Payload = make(map[string]any, len(j.Payload))
		for k, v := range j.Payload {
			c.Payload[k] = v
		}
	}
	return &c
}

// EnqueueResult reports whether a job was accepted.
type EnqueueResult struct {
	Accepted bool
	Reason   string
	Job      *Job
}

// ListFilter filters List.
type ListFilter struct {
	Statuses   []string
	SessionIDs []string
	UserID     *string
	Limit      int
	Page       int
	// SortBy is "created_at" (default) or "updated_at"; SortAsc flips order.
	SortBy  string
	SortAsc bool
}

// Stats summarises the queue.
type Stats struct {
	Counts                 map[string]int `json:"counts"`
	OldestQueuedAgeSeconds *int64         `json:"oldest_queued_age_seconds"`
}

// Store is the queue persistence contract (mirrors the Python stores).
type Store interface {
	Enqueue(ctx context.Context, job *Job, maxDepth int) (*EnqueueResult, error)
	// Claim atomically takes the oldest runnable ticket: queued, or running
	// with a stale lock and attempts left; session-blocked tickets are
	// skipped. It increments Attempt (the fencing generation).
	Claim(ctx context.Context, workerID string, lockGrace time.Duration, deploymentID *string) (*Job, error)
	Heartbeat(ctx context.Context, workerID string, jobIDs []string) (int, error)
	// Complete writes a terminal (or paused) status, fenced on worker+attempt.
	Complete(ctx context.Context, jobID, workerID string, attempt int, status string, errMsg *string) (bool, error)
	// RetryOrFail requeues (after delay) while attempts remain, else fails.
	// Returns StatusQueued, StatusFailed, or "" when fenced out.
	RetryOrFail(ctx context.Context, jobID, workerID string, attempt int, errMsg string, delay time.Duration) (string, error)
	// Cancel tombstones a queued or paused ticket.
	Cancel(ctx context.Context, jobID string) (bool, error)
	// Continue flips a paused ticket back to queued with payload["continue"].
	Continue(ctx context.Context, jobID string, continuePayload map[string]any) (string, *Job, error)
	SweepExhausted(ctx context.Context, lockGrace time.Duration, limit int) ([]*Job, error)
	AcquireSweep(ctx context.Context, jobID, workerID string, lockGrace time.Duration) (bool, error)
	SettleSwept(ctx context.Context, jobID, workerID, status string, errMsg *string) (bool, error)
	Get(ctx context.Context, jobID string) (*Job, error)
	// SessionBlocker returns the id of the ticket a queued job waits behind.
	SessionBlocker(ctx context.Context, jobID string) (string, error)
	CountQueued(ctx context.Context) (int, error)
	List(ctx context.Context, f ListFilter) ([]*Job, int, error)
	// Requeue grants one more execution to a failed/cancelled ticket.
	Requeue(ctx context.Context, jobID string) (bool, error)
	Stats(ctx context.Context) (*Stats, error)
	// Cleanup deletes terminal tickets completed more than olderThan ago.
	Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
}

// NewID returns a random 32-hex-char id.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ptr[T any](v T) *T { return &v }

func isTerminal(status string) bool {
	return status == StatusCompleted || status == StatusFailed || status == StatusCancelled
}

func validSettle(status string, allowPaused bool) bool {
	return isTerminal(status) || (allowPaused && status == StatusPaused)
}

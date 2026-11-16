package jobqueue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Outcome is what a Handler reports for a job that returned without error.
type Outcome struct {
	// Status is StatusCompleted (default when empty), StatusPaused,
	// StatusCancelled or StatusFailed (terminal, never retried).
	Status string
	Error  string
}

// Handler executes one claimed job. Returning an error retries the job
// (with backoff) while attempts remain; wrap it with Permanent to fail at once.
type Handler func(ctx context.Context, job *Job) (Outcome, error)

type permanentError struct{ err error }

func (p permanentError) Error() string { return p.err.Error() }
func (p permanentError) Unwrap() error { return p.err }

// Permanent marks err as non-retryable.
func Permanent(err error) error { return permanentError{err} }

// Config mirrors Python's QueueConfig durability fields.
type Config struct {
	// MaxConcurrency bounds jobs executing at once on this worker (default 32).
	MaxConcurrency int
	// MaxQueueDepth bounds queued jobs; Enqueue beyond it is rejected (default 1000, <0 = unbounded).
	MaxQueueDepth int
	// MaxAttempts stamped on new jobs (default 1: a crashed run fails visibly).
	MaxAttempts int
	// RetryDelay is the base backoff: attempt N waits up to base*2^(N-1) with full jitter, capped at 10x base (default 30s).
	RetryDelay time.Duration
	// Timeout per execution; 0 disables (default 1h).
	Timeout time.Duration
	// DeploymentID stamps enqueued jobs and restricts claims (claim affinity).
	DeploymentID *string
	// LockGrace before a silent worker's claims are reclaimed (default 60s).
	LockGrace time.Duration
	// PollInterval for an idle worker (default 1s); local Enqueue wakes it immediately.
	PollInterval time.Duration
	// Retention of terminal jobs (default 24h).
	Retention time.Duration
	// StopTimeout drain window on Stop; must be below LockGrace (default min(30s, LockGrace/2)).
	StopTimeout time.Duration
}

func (c *Config) defaults() {
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = 32
	}
	if c.MaxQueueDepth == 0 {
		c.MaxQueueDepth = 1000
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 1
	}
	if c.RetryDelay <= 0 {
		c.RetryDelay = 30 * time.Second
	}
	if c.Timeout == 0 {
		c.Timeout = time.Hour
	}
	if c.LockGrace <= 0 {
		c.LockGrace = 60 * time.Second
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.Retention <= 0 {
		c.Retention = 24 * time.Hour
	}
	if c.StopTimeout <= 0 || c.StopTimeout >= c.LockGrace {
		c.StopTimeout = c.LockGrace / 2
		if c.StopTimeout > 30*time.Second {
			c.StopTimeout = 30 * time.Second
		}
	}
}

type inflight struct {
	job      *Job
	cancel   context.CancelFunc
	reason   string // "cancel", "shutdown", "lease_lost"
	reasonMu sync.Mutex
}

func (f *inflight) stop(reason string) {
	f.reasonMu.Lock()
	if f.reason == "" {
		f.reason = reason
	}
	f.reasonMu.Unlock()
	f.cancel()
}

func (f *inflight) why() string {
	f.reasonMu.Lock()
	defer f.reasonMu.Unlock()
	return f.reason
}

// Worker claims and executes jobs from a Store.
type Worker struct {
	Store    Store
	Handler  Handler
	Config   Config
	WorkerID string
	Logger   *log.Logger

	mu          sync.Mutex
	running     map[string]*inflight
	wake        chan struct{}
	cancel      context.CancelFunc
	done        chan struct{}
	wg          sync.WaitGroup
	lastCleanup time.Time
	started     bool
}

// NewWorker creates a worker; cfg zero values take defaults.
func NewWorker(store Store, handler Handler, cfg Config) *Worker {
	cfg.defaults()
	return &Worker{Store: store, Handler: handler, Config: cfg, WorkerID: "worker-" + NewID()[:8],
		running: map[string]*inflight{}, wake: make(chan struct{}, 1)}
}

func (w *Worker) logf(format string, args ...any) {
	if w.Logger != nil {
		w.Logger.Printf(format, args...)
		return
	}
	log.Printf("jobqueue: "+format, args...)
}

// EnqueueOptions describes a job to enqueue.
type EnqueueOptions struct {
	ID               string // default random
	JobType          string // default "run"
	ComponentType    string
	ComponentID      string
	SessionID        string
	UserID           *string
	Payload          map[string]any
	SerializeSession bool
	IdempotencyKey   string
	// Delay postpones availability.
	Delay time.Duration
}

// NewJob builds a queued Job from options using cfg defaults.
func (w *Worker) NewJob(o EnqueueOptions) *Job {
	now := time.Now().Unix()
	j := &Job{ID: o.ID, JobType: o.JobType, ComponentType: o.ComponentType, ComponentID: o.ComponentID,
		SessionID: o.SessionID, UserID: o.UserID, DeploymentID: w.Config.DeploymentID, Payload: o.Payload,
		SerializeSession: o.SerializeSession, Status: StatusQueued, MaxAttempts: w.Config.MaxAttempts,
		AvailableAt: now + int64(o.Delay/time.Second), CreatedAt: now}
	if j.ID == "" {
		j.ID = NewID()
	}
	if j.JobType == "" {
		j.JobType = "run"
	}
	if o.IdempotencyKey != "" {
		j.IdempotencyKey = ptr(o.IdempotencyKey)
	}
	return j
}

// Enqueue stores a job and wakes this worker.
func (w *Worker) Enqueue(ctx context.Context, o EnqueueOptions) (*EnqueueResult, error) {
	depth := w.Config.MaxQueueDepth
	if depth < 0 {
		depth = 0
	}
	res, err := w.Store.Enqueue(ctx, w.NewJob(o), depth)
	if err == nil && res.Accepted {
		w.Wake()
	}
	return res, err
}

// Wake makes an idle worker poll immediately.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Cancel cancels a job: queued/paused tickets are tombstoned; a job running
// on this worker has its context cancelled and is settled as cancelled.
func (w *Worker) Cancel(ctx context.Context, jobID string) (bool, error) {
	ok, err := w.Store.Cancel(ctx, jobID)
	if err != nil || ok {
		return ok, err
	}
	w.mu.Lock()
	f := w.running[jobID]
	w.mu.Unlock()
	if f != nil {
		f.stop("cancel")
		return true, nil
	}
	return false, nil
}

// Continue re-queues a paused job with continuation input and wakes the worker.
func (w *Worker) Continue(ctx context.Context, jobID string, input map[string]any) (string, *Job, error) {
	outcome, j, err := w.Store.Continue(ctx, jobID, input)
	if err == nil && outcome == ContinueQueued {
		w.Wake()
	}
	return outcome, j, err
}

// Start launches the poll and heartbeat loops.
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	loopCtx, cancel := context.WithCancel(ctx)
	w.cancel, w.done = cancel, make(chan struct{})
	w.mu.Unlock()

	var loops sync.WaitGroup
	loops.Add(2)
	go func() { defer loops.Done(); w.pollLoop(loopCtx) }()
	go func() { defer loops.Done(); w.heartbeatLoop(loopCtx) }()
	go func() { loops.Wait(); close(w.done) }()
}

// Stop stops claiming, gives in-flight jobs StopTimeout to finish, then
// cancels stragglers (which are retried or failed per their budget).
func (w *Worker) Stop(ctx context.Context) error {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = false
	cancel, done := w.cancel, w.done
	w.mu.Unlock()
	cancel()
	<-done
	drained := make(chan struct{})
	go func() { w.wg.Wait(); close(drained) }()
	select {
	case <-drained:
		return nil
	case <-time.After(w.Config.StopTimeout):
	case <-ctx.Done():
	}
	w.mu.Lock()
	for _, f := range w.running {
		f.stop("shutdown")
	}
	w.mu.Unlock()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Drain waits until no job is executing on this worker.
func (w *Worker) Drain() { w.wg.Wait() }

func (w *Worker) pollLoop(ctx context.Context) {
	t := time.NewTicker(w.Config.PollInterval)
	defer t.Stop()
	for {
		w.ClaimBurst(ctx)
		w.Sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-w.wake:
		}
	}
}

// ClaimBurst claims jobs until none are runnable or capacity is full,
// starting each in its own goroutine. It returns the number claimed.
func (w *Worker) ClaimBurst(ctx context.Context) int {
	n := 0
	for ctx.Err() == nil {
		w.mu.Lock()
		full := len(w.running) >= w.Config.MaxConcurrency
		w.mu.Unlock()
		if full {
			break
		}
		job, err := w.Store.Claim(ctx, w.WorkerID, w.Config.LockGrace, w.Config.DeploymentID)
		if err != nil {
			w.logf("claim failed: %v", err)
			break
		}
		if job == nil {
			break
		}
		n++
		jobCtx, cancel := context.WithCancel(context.Background())
		f := &inflight{job: job, cancel: cancel}
		w.mu.Lock()
		w.running[job.ID] = f
		w.mu.Unlock()
		w.wg.Add(1)
		go w.execute(jobCtx, f)
	}
	return n
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	base := w.Config.RetryDelay
	d := base << uint(attempt-1)
	if attempt > 20 || d > 10*base || d <= 0 {
		d = 10 * base
	}
	return time.Duration(rand.Int63n(int64(d) + 1))
}

func (w *Worker) execute(ctx context.Context, f *inflight) {
	defer w.wg.Done()
	defer func() {
		w.mu.Lock()
		delete(w.running, f.job.ID)
		w.mu.Unlock()
		f.cancel()
		w.Wake() // a finished session head may unblock the next turn
	}()
	job := f.job
	runCtx := ctx
	if w.Config.Timeout > 0 {
		var c context.CancelFunc
		runCtx, c = context.WithTimeout(ctx, w.Config.Timeout)
		defer c()
	}
	out, err := w.safeHandle(runCtx, job)
	store := context.Background()
	timedOut := errors.Is(runCtx.Err(), context.DeadlineExceeded)

	switch reason := f.why(); {
	case reason == "lease_lost":
		return // another worker owns the ticket now; any write would be fenced out
	case reason == "cancel":
		w.settle(store, job, StatusCancelled, ptr("cancelled"))
		return
	case reason == "shutdown":
		w.retry(store, job, "worker shutting down")
		return
	case timedOut:
		w.retry(store, job, fmt.Sprintf("timed out after %s", w.Config.Timeout))
		return
	}
	if err != nil {
		var p permanentError
		if errors.As(err, &p) {
			w.settle(store, job, StatusFailed, ptr(err.Error()))
			return
		}
		w.retry(store, job, err.Error())
		return
	}
	status := out.Status
	if status == "" {
		status = StatusCompleted
	}
	var msg *string
	if out.Error != "" {
		msg = ptr(out.Error)
	}
	w.settle(store, job, status, msg)
}

func (w *Worker) safeHandle(ctx context.Context, job *Job) (out Outcome, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return w.Handler(ctx, job.Clone())
}

func (w *Worker) settle(ctx context.Context, job *Job, status string, msg *string) {
	if ok, err := w.Store.Complete(ctx, job.ID, w.WorkerID, job.Attempt, status, msg); err != nil || !ok {
		w.logf("job %s: settle %s fenced out or failed: %v", job.ID, status, err)
	}
}

func (w *Worker) retry(ctx context.Context, job *Job, msg string) {
	if _, err := w.Store.RetryOrFail(ctx, job.ID, w.WorkerID, job.Attempt, msg, w.retryDelay(job.Attempt)); err != nil {
		w.logf("job %s: retry-or-fail: %v", job.ID, err)
	}
}

func (w *Worker) heartbeatLoop(ctx context.Context) {
	interval := w.Config.LockGrace / 3
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Heartbeat(ctx)
		}
	}
}

// Heartbeat refreshes this worker's leases; jobs whose lease was lost
// (reclaimed by another worker) are cancelled locally.
func (w *Worker) Heartbeat(ctx context.Context) {
	w.mu.Lock()
	ids := make([]string, 0, len(w.running))
	for id := range w.running {
		ids = append(ids, id)
	}
	w.mu.Unlock()
	if len(ids) == 0 {
		return
	}
	n, err := w.Store.Heartbeat(ctx, w.WorkerID, ids)
	if err != nil {
		w.logf("heartbeat failed: %v", err)
		return
	}
	if n == len(ids) {
		return
	}
	for _, id := range ids {
		j, err := w.Store.Get(ctx, id)
		w.mu.Lock()
		f := w.running[id]
		w.mu.Unlock()
		if f == nil || (err != nil && !errors.Is(err, ErrNotFound)) {
			continue
		}
		if j == nil || j.Status != StatusRunning || j.LockedBy == nil || *j.LockedBy != w.WorkerID || j.Attempt != f.job.Attempt {
			w.logf("job %s: lease lost, cancelling local execution", id)
			f.stop("lease_lost")
		}
	}
}

// Sweep fails stale running jobs that exhausted their attempts (their
// worker died) and periodically deletes old terminal jobs.
func (w *Worker) Sweep(ctx context.Context) {
	jobs, err := w.Store.SweepExhausted(ctx, w.Config.LockGrace, 20)
	if err != nil {
		w.logf("sweep failed: %v", err)
	}
	for _, j := range jobs {
		if ok, err := w.Store.AcquireSweep(ctx, j.ID, w.WorkerID, w.Config.LockGrace); err != nil || !ok {
			continue
		}
		msg := "worker lost: lease expired with no attempts left"
		if _, err := w.Store.SettleSwept(ctx, j.ID, w.WorkerID, StatusFailed, &msg); err != nil {
			w.logf("settle swept %s: %v", j.ID, err)
		}
	}
	w.mu.Lock()
	due := time.Since(w.lastCleanup) > time.Hour
	if due {
		w.lastCleanup = time.Now()
	}
	w.mu.Unlock()
	if due {
		if _, err := w.Store.Cleanup(ctx, w.Config.Retention); err != nil {
			w.logf("cleanup failed: %v", err)
		}
	}
}

// Running returns ids of jobs executing on this worker.
func (w *Worker) Running() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	ids := make([]string, 0, len(w.running))
	for id := range w.running {
		ids = append(ids, id)
	}
	return ids
}

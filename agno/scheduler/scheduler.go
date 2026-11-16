package scheduler

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

// Scheduler polls a Store for due schedules and executes them through an
// Executor, recording one ScheduleRun per attempt. Several Scheduler
// instances may share one SQL store: claims are atomic.
type Scheduler struct {
	Store    Store
	Executor Executor
	// PollInterval between polls (default 15s).
	PollInterval time.Duration
	// WorkerID identifies this poller in schedule locks (default random).
	WorkerID string
	// MaxConcurrent bounds in-flight executions (default 10).
	MaxConcurrent int
	// LockGrace after which another worker may reclaim a locked schedule (default 5m).
	LockGrace time.Duration
	// Sleep waits between retries; nil uses a context-aware timer. Tests override it.
	Sleep func(ctx context.Context, d time.Duration) error
	// Logger receives operational messages; nil uses the standard logger.
	Logger *log.Logger

	mu       sync.Mutex
	execs    map[int64]*execution
	nextExec int64
	wg       sync.WaitGroup
	cancel   context.CancelFunc
	done     chan struct{}
	wake     chan struct{}
	inflight int
}

type execution struct {
	scheduleID string
	runID      string
	cancel     context.CancelFunc
}

// New creates a Scheduler with defaults.
func New(store Store, executor Executor) *Scheduler {
	return &Scheduler{Store: store, Executor: executor}
}

func (s *Scheduler) init() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.PollInterval <= 0 {
		s.PollInterval = 15 * time.Second
	}
	if s.WorkerID == "" {
		s.WorkerID = "worker-" + randomHex(4)
	}
	if s.MaxConcurrent <= 0 {
		s.MaxConcurrent = 10
	}
	if s.LockGrace <= 0 {
		s.LockGrace = 5 * time.Minute
	}
	if s.execs == nil {
		s.execs = map[int64]*execution{}
	}
	if s.wake == nil {
		s.wake = make(chan struct{}, 1)
	}
}

func (s *Scheduler) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
		return
	}
	log.Printf("scheduler: "+format, args...)
}

// Start launches the polling loop. It returns immediately.
func (s *Scheduler) Start(ctx context.Context) {
	s.init()
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel, s.done = cancel, make(chan struct{})
	done := s.done
	s.mu.Unlock()
	go func() {
		defer close(done)
		t := time.NewTicker(s.PollInterval)
		defer t.Stop()
		for {
			s.PollOnce(loopCtx)
			select {
			case <-loopCtx.Done():
				return
			case <-t.C:
			case <-s.wake:
			}
		}
	}()
}

// Wake forces an immediate poll on a running scheduler.
func (s *Scheduler) Wake() {
	s.init()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Stop stops polling, cancels in-flight executions (recorded as cancelled)
// and waits for them until ctx expires.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.init()
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel = nil
	for _, e := range s.execs {
		e.cancel()
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	waited := make(chan struct{})
	go func() { s.wg.Wait(); close(waited) }()
	select {
	case <-waited:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Wait blocks until every in-flight execution on this instance finishes.
func (s *Scheduler) Wait() { s.wg.Wait() }

// PollOnce claims every due schedule (up to MaxConcurrent in flight) and
// executes each in its own goroutine. It returns the number claimed.
func (s *Scheduler) PollOnce(ctx context.Context) int {
	s.init()
	n := 0
	for ctx.Err() == nil {
		s.mu.Lock()
		full := s.inflight >= s.MaxConcurrent
		if !full {
			s.inflight++
		}
		s.mu.Unlock()
		if full {
			break
		}
		sc, err := s.Store.ClaimDueSchedule(ctx, s.WorkerID, s.LockGrace)
		if err != nil || sc == nil {
			s.mu.Lock()
			s.inflight--
			s.mu.Unlock()
			if err != nil {
				s.logf("claim error: %v", err)
			}
			break
		}
		n++
		s.wg.Add(1)
		go func(sc *Schedule) {
			defer s.wg.Done()
			defer func() {
				s.mu.Lock()
				s.inflight--
				s.mu.Unlock()
			}()
			// Executions outlive the poll loop's context; Stop cancels them explicitly.
			if _, err := s.Execute(context.WithoutCancel(ctx), sc, true); err != nil {
				s.logf("schedule %s: %v", sc.ID, err)
			}
		}(sc)
	}
	return n
}

// Trigger runs a schedule immediately and synchronously, without touching
// its lock or next_run_at. userID scopes the lookup when non-nil.
func (s *Scheduler) Trigger(ctx context.Context, scheduleID string, userID *string) (*ScheduleRun, error) {
	s.init()
	sc, err := s.Store.GetSchedule(ctx, scheduleID, userID)
	if err != nil {
		return nil, err
	}
	if !sc.Enabled {
		return nil, ErrDisabled
	}
	s.wg.Add(1)
	defer s.wg.Done()
	return s.Execute(ctx, sc, false)
}

// Cancel cancels every in-flight execution of a schedule; the current
// attempt is recorded as cancelled and no retry follows. It returns how many
// executions were cancelled.
func (s *Scheduler) Cancel(scheduleID string) int {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.execs {
		if e.scheduleID == scheduleID {
			e.cancel()
			n++
		}
	}
	return n
}

// CancelRun cancels the in-flight execution whose current attempt is runID.
func (s *Scheduler) CancelRun(runID string) bool {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.execs {
		if e.runID == runID {
			e.cancel()
			return true
		}
	}
	return false
}

// Running returns the ids of schedules executing on this instance.
func (s *Scheduler) Running() []string {
	s.init()
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for _, e := range s.execs {
		ids = append(ids, e.scheduleID)
	}
	return ids
}

func (s *Scheduler) sleep(ctx context.Context, d time.Duration) error {
	if s.Sleep != nil {
		return s.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Execute runs sc with retries, persisting a run record per attempt. When
// release is true the claim is released and next_run_at recomputed (one-shot
// schedules are disabled after firing). It returns the last attempt's run.
func (s *Scheduler) Execute(ctx context.Context, sc *Schedule, release bool) (*ScheduleRun, error) {
	s.init()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	exec := &execution{scheduleID: sc.ID, cancel: cancel}
	s.mu.Lock()
	s.nextExec++
	key := s.nextExec
	s.execs[key] = exec
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.execs, key)
		s.mu.Unlock()
	}()
	// Bookkeeping writes must survive cancellation of the execution.
	store := context.WithoutCancel(ctx)
	if release {
		defer s.release(store, sc)
	}

	maxAttempts := sc.MaxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	delay := time.Duration(sc.RetryDelaySeconds) * time.Second
	if delay <= 0 {
		delay = 60 * time.Second
	}
	var last *ScheduleRun
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		now := time.Now().Unix()
		run := &ScheduleRun{
			ID: NewScheduleRunID(), ScheduleID: sc.ID, UserID: sc.UserID, Attempt: attempt,
			TriggeredAt: ptr(now), Status: RunStatusRunning, CreatedAt: now,
		}
		if err := s.Store.CreateRun(store, run); err != nil {
			return last, err
		}
		s.mu.Lock()
		exec.runID = run.ID
		s.mu.Unlock()
		last = run

		attemptCtx := ctx
		var attemptCancel context.CancelFunc = func() {}
		if sc.TimeoutSeconds > 0 {
			attemptCtx, attemptCancel = context.WithTimeout(ctx, time.Duration(sc.TimeoutSeconds)*time.Second)
		}
		res, err := s.safeExecute(attemptCtx, sc)
		timedOut := errors.Is(attemptCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
		attemptCancel()

		if res != nil {
			run.StatusCode = res.StatusCode
			run.Input, run.Output, run.Requirements = res.Input, res.Output, res.Requirements
			if res.RunID != "" {
				run.RunID = ptr(res.RunID)
			}
			if res.SessionID != "" {
				run.SessionID = ptr(res.SessionID)
			}
		}
		switch {
		case ctx.Err() != nil:
			run.Status, run.Error = RunStatusCancelled, ptr("Execution cancelled")
		case timedOut:
			run.Status, run.Error = RunStatusTimeout, ptr("Execution timed out")
		case err != nil:
			run.Status, run.Error = RunStatusFailed, ptr(err.Error())
		default:
			run.Status = RunStatusSuccess
			if res != nil && res.Status != "" {
				run.Status = res.Status
			}
			if res != nil && res.Error != "" {
				run.Error = ptr(res.Error)
			}
		}
		run.CompletedAt = ptr(time.Now().Unix())
		if uerr := s.Store.UpdateRun(store, run); uerr != nil {
			s.logf("update run %s: %v", run.ID, uerr)
		}
		if run.Status == RunStatusSuccess || run.Status == RunStatusPaused || run.Status == RunStatusCancelled {
			break
		}
		if attempt < maxAttempts {
			if s.sleep(ctx, delay) != nil {
				break
			}
		}
	}
	return last, nil
}

func (s *Scheduler) safeExecute(ctx context.Context, sc *Schedule) (res *Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("executor panic: " + toString(r))
		}
	}()
	if s.Executor == nil {
		return nil, errors.New("no executor configured")
	}
	return s.Executor.Execute(ctx, sc.Clone())
}

func (s *Scheduler) release(ctx context.Context, sc *Schedule) {
	r := Release{LockedAt: sc.LockedAt}
	if sc.LockedBy != nil {
		r.WorkerID = *sc.LockedBy
	}
	switch sc.Kind() {
	case KindOnce:
		r.Disable, r.DisabledReason = true, ptr("completed")
	default:
		var after *int64
		if sc.Kind() == KindInterval {
			after = sc.NextRunAt
		}
		next, err := ComputeNextRun(sc, after, true)
		if err != nil {
			s.logf("schedule %s: next run: %v; disabling", sc.ID, err)
			r.Disable, r.DisabledReason = true, ptr("next_run_error")
		}
		r.NextRunAt = next
	}
	if err := s.Store.ReleaseSchedule(ctx, sc.ID, r); err != nil {
		s.logf("release schedule %s: %v", sc.ID, err)
	}
}

func toString(v any) string {
	if e, ok := v.(error); ok {
		return e.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown panic"
}

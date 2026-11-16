package jobqueue

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Live statuses hold a session's place in line; holding statuses block it.
var (
	liveStatuses    = []string{StatusQueued, StatusRunning, StatusPaused}
	holdingStatuses = []string{StatusRunning, StatusPaused}
)

func in(s string, set []string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}

// sessionBlocker returns the id of the ticket job waits behind, or "".
func sessionBlocker(job *Job, others map[string]*Job) string {
	if !job.SerializeSession {
		return ""
	}
	var earliest *Job
	for _, o := range others {
		if o.ID == job.ID || o.SessionID != job.SessionID || !o.SerializeSession {
			continue
		}
		if in(o.Status, holdingStatuses) {
			return o.ID
		}
		if job.Status == StatusQueued && o.Status == StatusQueued &&
			(o.SessionSeq < job.SessionSeq || (o.SessionSeq == job.SessionSeq && o.ID < job.ID)) {
			if earliest == nil || o.SessionSeq < earliest.SessionSeq || (o.SessionSeq == earliest.SessionSeq && o.ID < earliest.ID) {
				earliest = o
			}
		}
	}
	if earliest != nil {
		return earliest.ID
	}
	return ""
}

func nextSessionSeq(job *Job, others map[string]*Job) int64 {
	var max int64
	for _, o := range others {
		if o.SerializeSession && o.SessionID == job.SessionID && in(o.Status, liveStatuses) && o.SessionSeq > max {
			max = o.SessionSeq
		}
	}
	return max + 1
}

// MemoryStore is a process-local, non-durable Store (tests, dev).
type MemoryStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{jobs: map[string]*Job{}} }

func sameUser(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *MemoryStore) Enqueue(_ context.Context, job *Job, maxDepth int) (*EnqueueResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job = job.Clone()
	if job.IdempotencyKey != nil && *job.IdempotencyKey == "" {
		job.IdempotencyKey = nil
	}
	if job.IdempotencyKey != nil {
		for _, e := range m.jobs {
			if e.IdempotencyKey != nil && *e.IdempotencyKey == *job.IdempotencyKey && sameUser(e.UserID, job.UserID) {
				return &EnqueueResult{Reason: ReasonDuplicate, Job: e.Clone()}, nil
			}
		}
	}
	if maxDepth > 0 {
		n := 0
		for _, e := range m.jobs {
			if e.Status == StatusQueued {
				n++
			}
		}
		if n >= maxDepth {
			return &EnqueueResult{Reason: ReasonQueueFull}, nil
		}
	}
	if _, ok := m.jobs[job.ID]; ok {
		return nil, errDuplicateID(job.ID)
	}
	if job.SerializeSession {
		job.SessionSeq = nextSessionSeq(job, m.jobs)
	}
	m.jobs[job.ID] = job
	return &EnqueueResult{Accepted: true, Job: job.Clone()}, nil
}

func (m *MemoryStore) Claim(_ context.Context, workerID string, lockGrace time.Duration, deploymentID *string) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	stale := now - int64(lockGrace/time.Second)
	var best *Job
	for _, j := range m.jobs {
		if j.AvailableAt > now || !(j.DeploymentID == nil || (deploymentID != nil && *j.DeploymentID == *deploymentID)) {
			continue
		}
		claimable := j.Status == StatusQueued ||
			(j.Status == StatusRunning && j.LockedAt != nil && *j.LockedAt <= stale && j.Attempt < j.MaxAttempts)
		if !claimable || sessionBlocker(j, m.jobs) != "" {
			continue
		}
		if best == nil || j.CreatedAt < best.CreatedAt || (j.CreatedAt == best.CreatedAt && j.ID < best.ID) {
			best = j
		}
	}
	if best == nil {
		return nil, nil
	}
	best.Status, best.LockedBy, best.LockedAt = StatusRunning, ptr(workerID), ptr(now)
	best.Attempt++
	best.UpdatedAt = ptr(now)
	return best.Clone(), nil
}

func (m *MemoryStore) Heartbeat(_ context.Context, workerID string, ids []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	n := 0
	for _, id := range ids {
		if j, ok := m.jobs[id]; ok && j.Status == StatusRunning && j.LockedBy != nil && *j.LockedBy == workerID {
			j.LockedAt = ptr(now)
			n++
		}
	}
	return n, nil
}

func (m *MemoryStore) owned(id, workerID string, attempt int) *Job {
	j, ok := m.jobs[id]
	if !ok || j.LockedBy == nil || *j.LockedBy != workerID || j.Attempt != attempt || j.Status != StatusRunning {
		return nil
	}
	return j
}

func (m *MemoryStore) settle(j *Job, status string, errMsg *string) {
	now := time.Now().Unix()
	j.Status, j.Error, j.LockedBy, j.LockedAt = status, errMsg, nil, nil
	j.CompletedAt, j.UpdatedAt = ptr(now), ptr(now)
	if status == StatusPaused {
		j.CompletedAt = nil
	}
}

func (m *MemoryStore) Complete(_ context.Context, id, workerID string, attempt int, status string, errMsg *string) (bool, error) {
	if !validSettle(status, true) {
		return false, errBadStatus(status)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.owned(id, workerID, attempt)
	if j == nil {
		return false, nil
	}
	m.settle(j, status, errMsg)
	return true, nil
}

func (m *MemoryStore) RetryOrFail(_ context.Context, id, workerID string, attempt int, errMsg string, delay time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.owned(id, workerID, attempt)
	if j == nil {
		return "", nil
	}
	now := time.Now().Unix()
	if j.Attempt < j.MaxAttempts {
		j.Status, j.Error, j.LockedBy, j.LockedAt = StatusQueued, ptr(errMsg), nil, nil
		j.AvailableAt, j.UpdatedAt = now+int64(delay/time.Second), ptr(now)
		return StatusQueued, nil
	}
	m.settle(j, StatusFailed, ptr(errMsg))
	return StatusFailed, nil
}

func (m *MemoryStore) Cancel(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || (j.Status != StatusQueued && j.Status != StatusPaused) {
		return false, nil
	}
	now := time.Now().Unix()
	j.Status, j.CompletedAt, j.UpdatedAt = StatusCancelled, ptr(now), ptr(now)
	return true, nil
}

func (m *MemoryStore) Continue(_ context.Context, id string, cont map[string]any) (string, *Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return ContinueConflict, nil, nil
	}
	if isTerminal(j.Status) {
		return ContinueConflict, j.Clone(), nil
	}
	if j.Status == StatusQueued || j.Status == StatusRunning {
		return ContinueAttach, j.Clone(), nil
	}
	now := time.Now().Unix()
	if j.Payload == nil {
		j.Payload = map[string]any{}
	}
	j.Payload["continue"] = cont
	j.Status, j.MaxAttempts, j.AvailableAt = StatusQueued, j.Attempt+1, now
	j.LockedBy, j.LockedAt, j.CompletedAt, j.UpdatedAt = nil, nil, nil, ptr(now)
	return ContinueQueued, j.Clone(), nil
}

func (m *MemoryStore) SweepExhausted(_ context.Context, lockGrace time.Duration, limit int) ([]*Job, error) {
	m.mu.Lock()
	stale := time.Now().Unix() - int64(lockGrace/time.Second)
	var out []*Job
	for _, j := range m.jobs {
		if j.Status == StatusRunning && j.LockedAt != nil && *j.LockedAt <= stale && j.Attempt >= j.MaxAttempts {
			out = append(out, j.Clone())
		}
	}
	m.mu.Unlock()
	sort.Slice(out, func(a, b int) bool { return *out[a].LockedAt < *out[b].LockedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) AcquireSweep(_ context.Context, id, workerID string, lockGrace time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	j, ok := m.jobs[id]
	if !ok || j.Status != StatusRunning || j.LockedAt == nil || *j.LockedAt > now-int64(lockGrace/time.Second) || j.Attempt < j.MaxAttempts {
		return false, nil
	}
	j.LockedBy, j.LockedAt, j.UpdatedAt = ptr(workerID), ptr(now), ptr(now)
	return true, nil
}

func (m *MemoryStore) SettleSwept(_ context.Context, id, workerID, status string, errMsg *string) (bool, error) {
	if !validSettle(status, true) {
		return false, errBadStatus(status)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.Status != StatusRunning || j.LockedBy == nil || *j.LockedBy != workerID {
		return false, nil
	}
	m.settle(j, status, errMsg)
	return true, nil
}

func (m *MemoryStore) Get(_ context.Context, id string) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return j.Clone(), nil
}

func (m *MemoryStore) SessionBlocker(_ context.Context, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.Status != StatusQueued {
		return "", nil
	}
	return sessionBlocker(j, m.jobs), nil
}

func (m *MemoryStore) CountQueued(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, j := range m.jobs {
		if j.Status == StatusQueued {
			n++
		}
	}
	return n, nil
}

func (m *MemoryStore) List(_ context.Context, f ListFilter) ([]*Job, int, error) {
	m.mu.Lock()
	var out []*Job
	for _, j := range m.jobs {
		if (len(f.Statuses) == 0 || in(j.Status, f.Statuses)) && (len(f.SessionIDs) == 0 || in(j.SessionID, f.SessionIDs)) &&
			(f.UserID == nil || (j.UserID != nil && *j.UserID == *f.UserID)) {
			out = append(out, j.Clone())
		}
	}
	m.mu.Unlock()
	key := func(j *Job) int64 {
		if f.SortBy == "updated_at" && j.UpdatedAt != nil {
			return *j.UpdatedAt
		}
		return j.CreatedAt
	}
	sort.SliceStable(out, func(a, b int) bool {
		ka, kb := key(out[a]), key(out[b])
		if ka == kb {
			return out[a].ID < out[b].ID
		}
		if f.SortAsc {
			return ka < kb
		}
		return ka > kb
	})
	limit, page := normPage(f.Limit, f.Page)
	start := (page - 1) * limit
	if start > len(out) {
		start = len(out)
	}
	end := start + limit
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], len(out), nil
}

func (m *MemoryStore) Requeue(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || (j.Status != StatusFailed && j.Status != StatusCancelled) {
		return false, nil
	}
	now := time.Now().Unix()
	if j.SerializeSession {
		j.SessionSeq = nextSessionSeq(j, m.jobs)
	}
	j.Status, j.MaxAttempts, j.AvailableAt = StatusQueued, j.Attempt+1, now
	j.LockedBy, j.LockedAt, j.CompletedAt, j.UpdatedAt = nil, nil, nil, ptr(now)
	return true, nil
}

func (m *MemoryStore) Stats(_ context.Context) (*Stats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	st := &Stats{Counts: map[string]int{}}
	for _, j := range m.jobs {
		st.Counts[j.Status]++
		if j.Status == StatusQueued {
			age := now - j.CreatedAt
			if st.OldestQueuedAgeSeconds == nil || age > *st.OldestQueuedAgeSeconds {
				st.OldestQueuedAgeSeconds = ptr(age)
			}
		}
	}
	return st, nil
}

func (m *MemoryStore) Cleanup(_ context.Context, olderThan time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Unix() - int64(olderThan/time.Second)
	n := 0
	for id, j := range m.jobs {
		if isTerminal(j.Status) && j.CompletedAt != nil && *j.CompletedAt <= cutoff {
			delete(m.jobs, id)
			n++
		}
	}
	return n, nil
}

func normPage(limit, page int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	return limit, page
}

var _ Store = (*MemoryStore)(nil)

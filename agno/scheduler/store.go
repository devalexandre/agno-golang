package scheduler

import (
	"context"
	"sort"
	"sync"
	"time"
)

// ListFilter filters schedule listings. Empty UserID means no owner scoping.
type ListFilter struct {
	UserID  *string
	Enabled *bool
	Limit   int
	Page    int
}

// Release describes how a claimed schedule is unlocked after execution.
type Release struct {
	NextRunAt *int64
	// Disable also disables the schedule (one-shot completed, cron failure).
	Disable        bool
	DisabledReason *string
	// WorkerID/LockedAt fence the release to the claim being released, so an
	// expired invocation cannot release a newer claim. Empty/nil skip fencing.
	WorkerID string
	LockedAt *int64
}

// Store persists schedules and their run history.
type Store interface {
	CreateSchedule(ctx context.Context, s *Schedule) error
	// GetSchedule returns ErrNotFound when missing or not owned by userID (when non-nil).
	GetSchedule(ctx context.Context, id string, userID *string) (*Schedule, error)
	GetScheduleByName(ctx context.Context, name string, userID *string) (*Schedule, error)
	ListSchedules(ctx context.Context, f ListFilter) ([]*Schedule, int, error)
	// UpdateSchedule replaces the mutable columns of s (matched by ID).
	UpdateSchedule(ctx context.Context, s *Schedule) error
	DeleteSchedule(ctx context.Context, id string) error
	// ClaimDueSchedule atomically locks one enabled schedule whose next_run_at
	// is due and which is unlocked or whose lock is older than lockGrace.
	ClaimDueSchedule(ctx context.Context, workerID string, lockGrace time.Duration) (*Schedule, error)
	ReleaseSchedule(ctx context.Context, id string, r Release) error

	CreateRun(ctx context.Context, r *ScheduleRun) error
	UpdateRun(ctx context.Context, r *ScheduleRun) error
	GetRun(ctx context.Context, id string, userID *string) (*ScheduleRun, error)
	ListRuns(ctx context.Context, scheduleID string, userID *string, limit, page int) ([]*ScheduleRun, int, error)
}

// MemoryStore is a process-local Store, for tests and single-process use.
type MemoryStore struct {
	mu        sync.Mutex
	schedules map[string]*Schedule
	runs      map[string]*ScheduleRun
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{schedules: map[string]*Schedule{}, runs: map[string]*ScheduleRun{}}
}

func ownerMatches(owner, filter *string) bool {
	if filter == nil {
		return true
	}
	return owner != nil && *owner == *filter
}

func sameOwner(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func paginate(limit, page, total int) (int, int) {
	if limit <= 0 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return start, end
}

func (m *MemoryStore) CreateSchedule(_ context.Context, s *Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.schedules {
		if e.Name == s.Name && sameOwner(e.UserID, s.UserID) {
			return ErrDuplicateName
		}
	}
	m.schedules[s.ID] = s.Clone()
	return nil
}

func (m *MemoryStore) GetSchedule(_ context.Context, id string, userID *string) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[id]
	if !ok || !ownerMatches(s.UserID, userID) {
		return nil, ErrNotFound
	}
	return s.Clone(), nil
}

func (m *MemoryStore) GetScheduleByName(_ context.Context, name string, userID *string) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.schedules {
		if s.Name == name && sameOwner(s.UserID, userID) {
			return s.Clone(), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemoryStore) ListSchedules(_ context.Context, f ListFilter) ([]*Schedule, int, error) {
	m.mu.Lock()
	var out []*Schedule
	for _, s := range m.schedules {
		if !ownerMatches(s.UserID, f.UserID) || (f.Enabled != nil && s.Enabled != *f.Enabled) {
			continue
		}
		out = append(out, s.Clone())
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].ID < out[j].ID
	})
	a, b := paginate(f.Limit, f.Page, len(out))
	return out[a:b], len(out), nil
}

func (m *MemoryStore) UpdateSchedule(_ context.Context, s *Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.schedules[s.ID]
	if !ok {
		return ErrNotFound
	}
	for _, e := range m.schedules {
		if e.ID != s.ID && e.Name == s.Name && sameOwner(e.UserID, cur.UserID) {
			return ErrDuplicateName
		}
	}
	n := s.Clone()
	n.LockedBy, n.LockedAt, n.UserID, n.CreatedAt = cur.LockedBy, cur.LockedAt, cur.UserID, cur.CreatedAt
	m.schedules[s.ID] = n
	return nil
}

func (m *MemoryStore) DeleteSchedule(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.schedules[id]; !ok {
		return ErrNotFound
	}
	delete(m.schedules, id)
	for rid, r := range m.runs {
		if r.ScheduleID == id {
			delete(m.runs, rid)
		}
	}
	return nil
}

func (m *MemoryStore) ClaimDueSchedule(_ context.Context, workerID string, lockGrace time.Duration) (*Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	stale := now - int64(lockGrace/time.Second)
	var best *Schedule
	for _, s := range m.schedules {
		if !s.Enabled || s.NextRunAt == nil || *s.NextRunAt > now {
			continue
		}
		if s.LockedBy != nil && (s.LockedAt == nil || *s.LockedAt > stale) {
			continue
		}
		if best == nil || *s.NextRunAt < *best.NextRunAt {
			best = s
		}
	}
	if best == nil {
		return nil, nil
	}
	best.LockedBy, best.LockedAt = ptr(workerID), ptr(now)
	return best.Clone(), nil
}

func (m *MemoryStore) ReleaseSchedule(_ context.Context, id string, r Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[id]
	if !ok {
		return ErrNotFound
	}
	if r.WorkerID != "" && (s.LockedBy == nil || *s.LockedBy != r.WorkerID) {
		return nil
	}
	if r.LockedAt != nil && (s.LockedAt == nil || *s.LockedAt != *r.LockedAt) {
		return nil
	}
	s.LockedBy, s.LockedAt, s.NextRunAt = nil, nil, r.NextRunAt
	if r.Disable {
		s.Enabled = false
		s.DisabledReason = r.DisabledReason
	}
	s.UpdatedAt = ptr(time.Now().Unix())
	return nil
}

func (m *MemoryStore) CreateRun(_ context.Context, r *ScheduleRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[r.ID] = r.Clone()
	return nil
}

func (m *MemoryStore) UpdateRun(_ context.Context, r *ScheduleRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.runs[r.ID]; !ok {
		return ErrNotFound
	}
	m.runs[r.ID] = r.Clone()
	return nil
}

func (m *MemoryStore) GetRun(_ context.Context, id string, userID *string) (*ScheduleRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok || !ownerMatches(r.UserID, userID) {
		return nil, ErrNotFound
	}
	return r.Clone(), nil
}

func (m *MemoryStore) ListRuns(_ context.Context, scheduleID string, userID *string, limit, page int) ([]*ScheduleRun, int, error) {
	m.mu.Lock()
	var out []*ScheduleRun
	for _, r := range m.runs {
		if r.ScheduleID == scheduleID && ownerMatches(r.UserID, userID) {
			out = append(out, r.Clone())
		}
	}
	m.mu.Unlock()
	// Run ids are time-ordered: newest first.
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	a, b := paginate(limit, page, len(out))
	return out[a:b], len(out), nil
}

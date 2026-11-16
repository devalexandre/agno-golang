package a2aserver

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"sync"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/os/component"
)

// TaskRecord is a stored task plus the Agno bookkeeping behind it.
type TaskRecord struct {
	Task        a2a.Task       `json:"task"`
	RunID       string         `json:"run_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"`
	ComponentID string         `json:"component_id"`
	Kind        component.Kind `json:"kind"`
	UpdatedAt   int64          `json:"updated_at"`
}

// TaskQuery filters TaskStore.List.
type TaskQuery struct {
	ComponentID string
	Kind        component.Kind
	ContextID   string
	UserID      string
	State       a2a.TaskState
	PageSize    int
	PageToken   string
}

// TaskStore persists A2A tasks (Python A2ATaskStore). Implementations must
// return copies: callers mutate what they get.
type TaskStore interface {
	Save(ctx context.Context, rec *TaskRecord) error
	// Get returns (nil, nil) when the task does not exist.
	Get(ctx context.Context, taskID string) (*TaskRecord, error)
	// List returns a page of tasks (newest first), the next page token and the total.
	List(ctx context.Context, q TaskQuery) ([]*TaskRecord, string, int, error)
	Delete(ctx context.Context, taskID string) error
}

// MemoryTaskStore is an in-process TaskStore.
type MemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string][]byte
	seq   map[string]int64
	next  int64
}

// NewMemoryTaskStore returns an empty in-memory store.
func NewMemoryTaskStore() *MemoryTaskStore {
	return &MemoryTaskStore{tasks: map[string][]byte{}, seq: map[string]int64{}}
}

func (m *MemoryTaskStore) Save(_ context.Context, rec *TaskRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.seq[rec.Task.ID]; !ok {
		m.next++
		m.seq[rec.Task.ID] = m.next
	}
	m.tasks[rec.Task.ID] = b
	return nil
}

func (m *MemoryTaskStore) Get(_ context.Context, id string) (*TaskRecord, error) {
	m.mu.RLock()
	b, ok := m.tasks[id]
	m.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	var rec TaskRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (m *MemoryTaskStore) List(ctx context.Context, q TaskQuery) ([]*TaskRecord, string, int, error) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.tasks))
	for id := range m.tasks {
		ids = append(ids, id)
	}
	seq := make(map[string]int64, len(ids))
	for _, id := range ids {
		seq[id] = m.seq[id]
	}
	m.mu.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return seq[ids[i]] > seq[ids[j]] })
	var all []*TaskRecord
	for _, id := range ids {
		rec, err := m.Get(ctx, id)
		if err != nil || rec == nil {
			continue
		}
		if (q.ComponentID != "" && rec.ComponentID != q.ComponentID) || (q.Kind != "" && rec.Kind != q.Kind) ||
			(q.ContextID != "" && rec.Task.ContextID != q.ContextID) || (q.UserID != "" && rec.UserID != q.UserID) ||
			(q.State != "" && rec.Task.Status.State != q.State) {
			continue
		}
		all = append(all, rec)
	}
	size := q.PageSize
	if size <= 0 || size > 100 {
		size = 50
	}
	start, _ := strconv.Atoi(q.PageToken)
	if start < 0 || start > len(all) {
		start = len(all)
	}
	end := start + size
	next := ""
	if end < len(all) {
		next = strconv.Itoa(end)
	} else {
		end = len(all)
	}
	return all[start:end], next, len(all), nil
}

func (m *MemoryTaskStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	delete(m.tasks, id)
	delete(m.seq, id)
	m.mu.Unlock()
	return nil
}

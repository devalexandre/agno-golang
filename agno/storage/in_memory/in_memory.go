// Package inmemory provides an in-memory implementation of storage.DB.
// It is ideal for development, tests and ephemeral sessions — no external
// dependencies, fully concurrent-safe, and lost on process exit.
package inmemory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// InMemoryDB is a concurrency-safe in-memory storage.DB implementation.
type InMemoryDB struct {
	mu       sync.RWMutex
	sessions map[string]*storage.AgentSession
	runs     map[string]*storage.AgentRun
}

// New creates a new in-memory database.
func New() *InMemoryDB {
	return &InMemoryDB{
		sessions: make(map[string]*storage.AgentSession),
		runs:     make(map[string]*storage.AgentRun),
	}
}

// cloneSession returns a shallow copy so callers cannot mutate stored state.
func cloneSession(s *storage.AgentSession) *storage.AgentSession {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

func cloneRun(r *storage.AgentRun) *storage.AgentRun {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}

// CreateSession stores a new session.
func (d *InMemoryDB) CreateSession(_ context.Context, session *storage.AgentSession) error {
	if session == nil || session.SessionID == "" {
		return fmt.Errorf("inmemory: session and session_id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.sessions[session.SessionID]; exists {
		return fmt.Errorf("inmemory: session %q already exists", session.SessionID)
	}
	d.sessions[session.SessionID] = cloneSession(session)
	return nil
}

// ReadSession returns a session by ID, or (nil, nil) when not found.
func (d *InMemoryDB) ReadSession(_ context.Context, sessionID string) (*storage.AgentSession, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	return cloneSession(s), nil
}

// UpdateSession updates an existing session (upserts if absent).
func (d *InMemoryDB) UpdateSession(_ context.Context, session *storage.AgentSession) error {
	if session == nil || session.SessionID == "" {
		return fmt.Errorf("inmemory: session and session_id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions[session.SessionID] = cloneSession(session)
	return nil
}

// DeleteSession removes a session by ID.
func (d *InMemoryDB) DeleteSession(_ context.Context, sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.sessions, sessionID)
	return nil
}

// GetAllSessionIDs returns all session IDs for a user (sorted for determinism).
func (d *InMemoryDB) GetAllSessionIDs(_ context.Context, userID string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var ids []string
	for id, s := range d.sessions {
		if userID == "" || s.UserID == userID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// GetAllSessions returns all sessions for a user (sorted by SessionID).
func (d *InMemoryDB) GetAllSessions(_ context.Context, userID string) ([]*storage.AgentSession, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []*storage.AgentSession
	for _, s := range d.sessions {
		if userID == "" || s.UserID == userID {
			out = append(out, cloneSession(s))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out, nil
}

// CreateRun stores a new run.
func (d *InMemoryDB) CreateRun(_ context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("inmemory: run and id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.runs[run.ID]; exists {
		return fmt.Errorf("inmemory: run %q already exists", run.ID)
	}
	d.runs[run.ID] = cloneRun(run)
	return nil
}

// ReadRun returns a run by ID, or (nil, nil) when not found.
func (d *InMemoryDB) ReadRun(_ context.Context, runID string) (*storage.AgentRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r, ok := d.runs[runID]
	if !ok {
		return nil, nil
	}
	return cloneRun(r), nil
}

// UpdateRun updates an existing run (upserts if absent).
func (d *InMemoryDB) UpdateRun(_ context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("inmemory: run and id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.runs[run.ID] = cloneRun(run)
	return nil
}

// DeleteRun removes a run by ID.
func (d *InMemoryDB) DeleteRun(_ context.Context, runID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.runs, runID)
	return nil
}

// GetRunsForSession returns all runs for a session (sorted by ID).
func (d *InMemoryDB) GetRunsForSession(_ context.Context, sessionID string) ([]*storage.AgentRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []*storage.AgentRun
	for _, r := range d.runs {
		if r.SessionID == sessionID {
			out = append(out, cloneRun(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// CreateTables is a no-op for the in-memory backend.
func (d *InMemoryDB) CreateTables(_ context.Context) error { return nil }

// UpgradeSchema is a no-op for the in-memory backend.
func (d *InMemoryDB) UpgradeSchema(_ context.Context) error { return nil }

// DropTables clears all stored data.
func (d *InMemoryDB) DropTables(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions = make(map[string]*storage.AgentSession)
	d.runs = make(map[string]*storage.AgentRun)
	return nil
}

// Ensure InMemoryDB satisfies the storage.DB interface.
var _ storage.DB = (*InMemoryDB)(nil)

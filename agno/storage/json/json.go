// Package jsonstore provides a file-backed JSON implementation of storage.DB.
// State is held in memory and persisted to a single JSON file on each mutation.
// It has no external dependencies and is suitable for small, single-process apps.
package jsonstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// JSONDB is a concurrency-safe, file-backed storage.DB implementation.
type JSONDB struct {
	mu   sync.RWMutex
	path string
	data store
}

type store struct {
	Sessions map[string]*storage.AgentSession `json:"sessions"`
	Runs     map[string]*storage.AgentRun     `json:"runs"`
}

// New creates a JSON-backed database persisting to path. If the file exists it
// is loaded; otherwise it is created on the first write.
func New(path string) (*JSONDB, error) {
	if path == "" {
		return nil, fmt.Errorf("jsonstore: path is required")
	}
	d := &JSONDB{
		path: path,
		data: store{
			Sessions: make(map[string]*storage.AgentSession),
			Runs:     make(map[string]*storage.AgentRun),
		},
	}
	if err := d.load(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *JSONDB) load() error {
	raw, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh store
		}
		return fmt.Errorf("jsonstore: read %s: %w", d.path, err)
	}
	if len(raw) == 0 {
		return nil
	}
	var s store
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("jsonstore: parse %s: %w", d.path, err)
	}
	if s.Sessions == nil {
		s.Sessions = make(map[string]*storage.AgentSession)
	}
	if s.Runs == nil {
		s.Runs = make(map[string]*storage.AgentRun)
	}
	d.data = s
	return nil
}

// save writes the current state atomically (write temp + rename).
func (d *JSONDB) save() error {
	if dir := filepath.Dir(d.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("jsonstore: mkdir %s: %w", dir, err)
		}
	}
	raw, err := json.MarshalIndent(d.data, "", "  ")
	if err != nil {
		return fmt.Errorf("jsonstore: marshal: %w", err)
	}
	tmp := d.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("jsonstore: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, d.path); err != nil {
		return fmt.Errorf("jsonstore: rename %s: %w", d.path, err)
	}
	return nil
}

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

// CreateSession stores a new session and persists.
func (d *JSONDB) CreateSession(_ context.Context, session *storage.AgentSession) error {
	if session == nil || session.SessionID == "" {
		return fmt.Errorf("jsonstore: session and session_id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.data.Sessions[session.SessionID]; exists {
		return fmt.Errorf("jsonstore: session %q already exists", session.SessionID)
	}
	d.data.Sessions[session.SessionID] = cloneSession(session)
	return d.save()
}

// ReadSession returns a session by ID, or (nil, nil) when not found.
func (d *JSONDB) ReadSession(_ context.Context, sessionID string) (*storage.AgentSession, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.data.Sessions[sessionID]
	if !ok {
		return nil, nil
	}
	return cloneSession(s), nil
}

// UpdateSession updates an existing session (upserts if absent) and persists.
func (d *JSONDB) UpdateSession(_ context.Context, session *storage.AgentSession) error {
	if session == nil || session.SessionID == "" {
		return fmt.Errorf("jsonstore: session and session_id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data.Sessions[session.SessionID] = cloneSession(session)
	return d.save()
}

// DeleteSession removes a session by ID and persists.
func (d *JSONDB) DeleteSession(_ context.Context, sessionID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.data.Sessions, sessionID)
	return d.save()
}

// GetAllSessionIDs returns all session IDs for a user (sorted).
func (d *JSONDB) GetAllSessionIDs(_ context.Context, userID string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var ids []string
	for id, s := range d.data.Sessions {
		if userID == "" || s.UserID == userID {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// GetAllSessions returns all sessions for a user (sorted by SessionID).
func (d *JSONDB) GetAllSessions(_ context.Context, userID string) ([]*storage.AgentSession, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []*storage.AgentSession
	for _, s := range d.data.Sessions {
		if userID == "" || s.UserID == userID {
			out = append(out, cloneSession(s))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out, nil
}

// CreateRun stores a new run and persists.
func (d *JSONDB) CreateRun(_ context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("jsonstore: run and id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.data.Runs[run.ID]; exists {
		return fmt.Errorf("jsonstore: run %q already exists", run.ID)
	}
	d.data.Runs[run.ID] = cloneRun(run)
	return d.save()
}

// ReadRun returns a run by ID, or (nil, nil) when not found.
func (d *JSONDB) ReadRun(_ context.Context, runID string) (*storage.AgentRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r, ok := d.data.Runs[runID]
	if !ok {
		return nil, nil
	}
	return cloneRun(r), nil
}

// UpdateRun updates an existing run (upserts if absent) and persists.
func (d *JSONDB) UpdateRun(_ context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("jsonstore: run and id are required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data.Runs[run.ID] = cloneRun(run)
	return d.save()
}

// DeleteRun removes a run by ID and persists.
func (d *JSONDB) DeleteRun(_ context.Context, runID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.data.Runs, runID)
	return d.save()
}

// GetRunsForSession returns all runs for a session (sorted by ID).
func (d *JSONDB) GetRunsForSession(_ context.Context, sessionID string) ([]*storage.AgentRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []*storage.AgentRun
	for _, r := range d.data.Runs {
		if r.SessionID == sessionID {
			out = append(out, cloneRun(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// CreateTables ensures the backing file exists.
func (d *JSONDB) CreateTables(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.save()
}

// UpgradeSchema is a no-op for the JSON backend.
func (d *JSONDB) UpgradeSchema(_ context.Context) error { return nil }

// DropTables clears all data and persists an empty store.
func (d *JSONDB) DropTables(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.data = store{
		Sessions: make(map[string]*storage.AgentSession),
		Runs:     make(map[string]*storage.AgentRun),
	}
	return d.save()
}

// Ensure JSONDB satisfies the storage.DB interface.
var _ storage.DB = (*JSONDB)(nil)

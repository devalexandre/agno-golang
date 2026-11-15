package os

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultDBID is the id of the in-memory database AgentOS creates when no
// Database is configured. It matches the db_id the Go AgentOS has always
// advertised in /config.
const DefaultDBID = "agno-storage"

// Session types (mirrors Python agno.db.base.SessionType).
const (
	SessionTypeAgent    = "agent"
	SessionTypeTeam     = "team"
	SessionTypeWorkflow = "workflow"
)

// SessionRecord is the persisted shape of an agent, team or workflow session.
type SessionRecord struct {
	SessionID    string                 `json:"session_id"`
	SessionType  string                 `json:"session_type"`
	AgentID      string                 `json:"agent_id,omitempty"`
	TeamID       string                 `json:"team_id,omitempty"`
	WorkflowID   string                 `json:"workflow_id,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	SessionName  string                 `json:"session_name,omitempty"`
	SessionState map[string]interface{} `json:"session_state,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Summary      map[string]interface{} `json:"summary,omitempty"`
	Runs         []*RunRecord           `json:"runs,omitempty"`
	CreatedAt    int64                  `json:"created_at"`
	UpdatedAt    int64                  `json:"updated_at"`
}

// ComponentID returns the agent, team or workflow id of the session.
func (s *SessionRecord) ComponentID() string {
	switch s.SessionType {
	case SessionTypeTeam:
		return s.TeamID
	case SessionTypeWorkflow:
		return s.WorkflowID
	}
	return s.AgentID
}

// RunRecord is a run stored inside a session.
type RunRecord struct {
	RunID             string                   `json:"run_id"`
	RunIndex          int                      `json:"run_index"`
	ParentRunID       string                   `json:"parent_run_id,omitempty"`
	AgentID           string                   `json:"agent_id,omitempty"`
	TeamID            string                   `json:"team_id,omitempty"`
	WorkflowID        string                   `json:"workflow_id,omitempty"`
	UserID            string                   `json:"user_id,omitempty"`
	SessionID         string                   `json:"session_id"`
	Status            string                   `json:"status,omitempty"`
	CancellationStage string                   `json:"cancellation_stage,omitempty"`
	RunInput          string                   `json:"run_input,omitempty"`
	Content           interface{}              `json:"content,omitempty"`
	Metrics           map[string]interface{}   `json:"metrics,omitempty"`
	Events            []map[string]interface{} `json:"events,omitempty"`
	CreatedAt         int64                    `json:"created_at"`
	UpdatedAt         int64                    `json:"updated_at"`
}

// UserMemory is a stored user memory (mirrors Python agno.db.schemas.UserMemory).
type UserMemory struct {
	MemoryID  string   `json:"memory_id"`
	Memory    string   `json:"memory"`
	Topics    []string `json:"topics,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
	AgentID   string   `json:"agent_id,omitempty"`
	TeamID    string   `json:"team_id,omitempty"`
	Input     string   `json:"input,omitempty"`
	UpdatedAt int64    `json:"updated_at"`
}

// EvalRunRecord is a stored evaluation run (mirrors Python EvalRunRecord).
type EvalRunRecord struct {
	RunID                  string                 `json:"run_id"`
	Name                   string                 `json:"name,omitempty"`
	EvalType               string                 `json:"eval_type"`
	AgentID                string                 `json:"agent_id,omitempty"`
	TeamID                 string                 `json:"team_id,omitempty"`
	WorkflowID             string                 `json:"workflow_id,omitempty"`
	ModelID                string                 `json:"model_id,omitempty"`
	ModelProvider          string                 `json:"model_provider,omitempty"`
	EvaluatedComponentName string                 `json:"evaluated_component_name,omitempty"`
	UserID                 string                 `json:"user_id,omitempty"`
	EvalData               map[string]interface{} `json:"eval_data"`
	EvalInput              map[string]interface{} `json:"eval_input,omitempty"`
	CreatedAt              int64                  `json:"created_at"`
	UpdatedAt              int64                  `json:"updated_at"`
}

// DayMetrics are the metrics aggregated for one day (mirrors Python DayAggregatedMetrics).
type DayMetrics struct {
	ID                    string                   `json:"id"`
	AgentRunsCount        int                      `json:"agent_runs_count"`
	AgentSessionsCount    int                      `json:"agent_sessions_count"`
	TeamRunsCount         int                      `json:"team_runs_count"`
	TeamSessionsCount     int                      `json:"team_sessions_count"`
	WorkflowRunsCount     int                      `json:"workflow_runs_count"`
	WorkflowSessionsCount int                      `json:"workflow_sessions_count"`
	UsersCount            int                      `json:"users_count"`
	TokenMetrics          map[string]interface{}   `json:"token_metrics"`
	ModelMetrics          []map[string]interface{} `json:"model_metrics"`
	Date                  time.Time                `json:"date"`
	CreatedAt             time.Time                `json:"created_at"`
	UpdatedAt             time.Time                `json:"updated_at"`
}

// SessionQuery selects sessions. Zero values mean "no filter".
type SessionQuery struct {
	SessionType string
	ComponentID string
	UserID      string
	SessionName string // partial, case-insensitive
	// Filter is an extra predicate (used by the advanced session search).
	Filter    func(*SessionRecord) bool
	SortBy    string // created_at (default) or updated_at
	SortOrder string // desc (default) or asc
	Limit     int    // 0 = no limit
	Page      int    // 1-based
}

// MemoryQuery selects memories.
type MemoryQuery struct {
	UserID        string
	AgentID       string
	TeamID        string
	Topics        []string
	SearchContent string
	SortBy        string // updated_at (default)
	SortOrder     string
	Limit         int
	Page          int
}

// EvalQuery selects evaluation runs.
type EvalQuery struct {
	AgentID    string
	TeamID     string
	WorkflowID string
	ModelID    string
	EvalTypes  []string
	FilterType string // agent, team or workflow
	SortBy     string
	SortOrder  string
	Limit      int
	Page       int
}

// Database is the persistence contract of the AgentOS routers. It mirrors the
// subset of Python's BaseDb that the session, memory, eval and metrics routers
// use. Lookups of a missing row return (nil, nil).
type Database interface {
	GetID() string

	UpsertSession(ctx context.Context, session *SessionRecord) error
	GetSession(ctx context.Context, sessionID string) (*SessionRecord, error)
	ListSessions(ctx context.Context, q SessionQuery) ([]*SessionRecord, int, error)
	DeleteSessions(ctx context.Context, sessionIDs []string, userID string) error

	UpsertMemory(ctx context.Context, memory *UserMemory) error
	GetMemory(ctx context.Context, memoryID, userID string) (*UserMemory, error)
	ListMemories(ctx context.Context, q MemoryQuery) ([]*UserMemory, int, error)
	DeleteMemories(ctx context.Context, memoryIDs []string, userID string) error

	UpsertEvalRun(ctx context.Context, run *EvalRunRecord) error
	GetEvalRun(ctx context.Context, runID string) (*EvalRunRecord, error)
	ListEvalRuns(ctx context.Context, q EvalQuery) ([]*EvalRunRecord, int, error)
	DeleteEvalRuns(ctx context.Context, runIDs []string) error

	UpsertMetrics(ctx context.Context, metrics []*DayMetrics) error
	ListMetrics(ctx context.Context, start, end *time.Time) ([]*DayMetrics, error)
}

// InMemoryDB is a Database kept in process memory.
type InMemoryDB struct {
	id       string
	mu       sync.RWMutex
	sessions map[string]*SessionRecord
	memories map[string]*UserMemory
	evals    map[string]*EvalRunRecord
	metrics  map[string]*DayMetrics // keyed by YYYY-MM-DD
}

// NewInMemoryDB creates an in-memory AgentOS database.
func NewInMemoryDB(id string) *InMemoryDB {
	if id == "" {
		id = DefaultDBID
	}
	return &InMemoryDB{
		id:       id,
		sessions: map[string]*SessionRecord{},
		memories: map[string]*UserMemory{},
		evals:    map[string]*EvalRunRecord{},
		metrics:  map[string]*DayMetrics{},
	}
}

// GetID returns the database id.
func (d *InMemoryDB) GetID() string { return d.id }

func cloneSessionRecord(s *SessionRecord) *SessionRecord {
	if s == nil {
		return nil
	}
	c := *s
	c.Runs = make([]*RunRecord, len(s.Runs))
	for i, r := range s.Runs {
		rc := *r
		rc.Events = append([]map[string]interface{}(nil), r.Events...)
		c.Runs[i] = &rc
	}
	return &c
}

func (d *InMemoryDB) UpsertSession(_ context.Context, session *SessionRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessions[session.SessionID] = cloneSessionRecord(session)
	return nil
}

func (d *InMemoryDB) GetSession(_ context.Context, sessionID string) (*SessionRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return cloneSessionRecord(d.sessions[sessionID]), nil
}

func (d *InMemoryDB) ListSessions(_ context.Context, q SessionQuery) ([]*SessionRecord, int, error) {
	d.mu.RLock()
	out := make([]*SessionRecord, 0, len(d.sessions))
	for _, s := range d.sessions {
		if q.SessionType != "" && s.SessionType != q.SessionType {
			continue
		}
		if q.ComponentID != "" && s.ComponentID() != q.ComponentID {
			continue
		}
		if q.UserID != "" && s.UserID != q.UserID {
			continue
		}
		if q.SessionName != "" && !strings.Contains(strings.ToLower(s.SessionName), strings.ToLower(q.SessionName)) {
			continue
		}
		if q.Filter != nil && !q.Filter(s) {
			continue
		}
		out = append(out, cloneSessionRecord(s))
	}
	d.mu.RUnlock()

	desc := !strings.EqualFold(q.SortOrder, "asc")
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].CreatedAt, out[j].CreatedAt
		if q.SortBy == "updated_at" {
			a, b = out[i].UpdatedAt, out[j].UpdatedAt
		}
		if a == b {
			return out[i].SessionID < out[j].SessionID
		}
		if desc {
			return a > b
		}
		return a < b
	})
	total := len(out)
	return paginate(out, q.Limit, q.Page), total, nil
}

func (d *InMemoryDB) DeleteSessions(_ context.Context, sessionIDs []string, userID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range sessionIDs {
		if s, ok := d.sessions[id]; ok && (userID == "" || s.UserID == userID) {
			delete(d.sessions, id)
		}
	}
	return nil
}

func (d *InMemoryDB) UpsertMemory(_ context.Context, memory *UserMemory) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := *memory
	c.Topics = append([]string(nil), memory.Topics...)
	d.memories[memory.MemoryID] = &c
	return nil
}

func (d *InMemoryDB) GetMemory(_ context.Context, memoryID, userID string) (*UserMemory, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	m, ok := d.memories[memoryID]
	if !ok || (userID != "" && m.UserID != userID) {
		return nil, nil
	}
	c := *m
	return &c, nil
}

func (d *InMemoryDB) ListMemories(_ context.Context, q MemoryQuery) ([]*UserMemory, int, error) {
	d.mu.RLock()
	out := make([]*UserMemory, 0, len(d.memories))
	for _, m := range d.memories {
		if q.UserID != "" && m.UserID != q.UserID {
			continue
		}
		if q.AgentID != "" && m.AgentID != q.AgentID {
			continue
		}
		if q.TeamID != "" && m.TeamID != q.TeamID {
			continue
		}
		if q.SearchContent != "" && !strings.Contains(strings.ToLower(m.Memory), strings.ToLower(q.SearchContent)) {
			continue
		}
		if len(q.Topics) > 0 && !hasAnyTopic(m.Topics, q.Topics) {
			continue
		}
		c := *m
		out = append(out, &c)
	}
	d.mu.RUnlock()
	desc := !strings.EqualFold(q.SortOrder, "asc")
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].MemoryID < out[j].MemoryID
		}
		if desc {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].UpdatedAt < out[j].UpdatedAt
	})
	total := len(out)
	return paginate(out, q.Limit, q.Page), total, nil
}

func hasAnyTopic(have, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if h == w {
				return true
			}
		}
	}
	return false
}

func (d *InMemoryDB) DeleteMemories(_ context.Context, memoryIDs []string, userID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range memoryIDs {
		if m, ok := d.memories[id]; ok && (userID == "" || m.UserID == userID) {
			delete(d.memories, id)
		}
	}
	return nil
}

func (d *InMemoryDB) UpsertEvalRun(_ context.Context, run *EvalRunRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	c := *run
	d.evals[run.RunID] = &c
	return nil
}

func (d *InMemoryDB) GetEvalRun(_ context.Context, runID string) (*EvalRunRecord, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r, ok := d.evals[runID]
	if !ok {
		return nil, nil
	}
	c := *r
	return &c, nil
}

func (d *InMemoryDB) ListEvalRuns(_ context.Context, q EvalQuery) ([]*EvalRunRecord, int, error) {
	d.mu.RLock()
	out := make([]*EvalRunRecord, 0, len(d.evals))
	for _, r := range d.evals {
		if q.AgentID != "" && r.AgentID != q.AgentID {
			continue
		}
		if q.TeamID != "" && r.TeamID != q.TeamID {
			continue
		}
		if q.WorkflowID != "" && r.WorkflowID != q.WorkflowID {
			continue
		}
		if q.ModelID != "" && r.ModelID != q.ModelID {
			continue
		}
		if len(q.EvalTypes) > 0 && !containsString(q.EvalTypes, r.EvalType) {
			continue
		}
		switch q.FilterType {
		case "agent":
			if r.AgentID == "" {
				continue
			}
		case "team":
			if r.TeamID == "" {
				continue
			}
		case "workflow":
			if r.WorkflowID == "" {
				continue
			}
		}
		c := *r
		out = append(out, &c)
	}
	d.mu.RUnlock()
	desc := !strings.EqualFold(q.SortOrder, "asc")
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].CreatedAt, out[j].CreatedAt
		if q.SortBy == "updated_at" {
			a, b = out[i].UpdatedAt, out[j].UpdatedAt
		}
		if a == b {
			return out[i].RunID < out[j].RunID
		}
		if desc {
			return a > b
		}
		return a < b
	})
	total := len(out)
	return paginate(out, q.Limit, q.Page), total, nil
}

func (d *InMemoryDB) DeleteEvalRuns(_ context.Context, runIDs []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range runIDs {
		delete(d.evals, id)
	}
	return nil
}

func (d *InMemoryDB) UpsertMetrics(_ context.Context, metrics []*DayMetrics) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, m := range metrics {
		c := *m
		d.metrics[m.Date.UTC().Format("2006-01-02")] = &c
	}
	return nil
}

func (d *InMemoryDB) ListMetrics(_ context.Context, start, end *time.Time) ([]*DayMetrics, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*DayMetrics, 0, len(d.metrics))
	for _, m := range d.metrics {
		if start != nil && m.Date.Before(*start) {
			continue
		}
		if end != nil && m.Date.After(*end) {
			continue
		}
		c := *m
		out = append(out, &c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// paginate returns page `page` (1-based; 0 is treated as 1) of `limit` items.
func paginate[T any](items []T, limit, page int) []T {
	if limit <= 0 {
		return items
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= len(items) {
		return []T{}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// Package kvstore implements the full storage contract (storage.DB,
// storage.Storage, storage.KnowledgeStorage, storage.SessionSearcher and
// storage.MetricsStore) on top of a minimal document Backend.
//
// Document databases and object stores (Redis/Valkey, MongoDB, DynamoDB,
// Firestore, GCS, SurrealDB, ClickHouse) only implement Backend — a keyed
// JSON document store with optional equality filters — and share one
// implementation of sessions, runs, knowledge, session search and metrics.
// This mirrors the Python key-value adapters, which also filter, sort and
// aggregate in the client.
package kvstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// Collections used by the store.
const (
	CollSessions           = "sessions"
	CollRuns               = "runs"
	CollTeamSessions       = "team_sessions"
	CollWorkflowSessions   = "workflow_sessions"
	CollWorkflowV2Sessions = "workflow_v2_sessions"
	CollKnowledge          = "knowledge"
	CollComponentMetrics   = "component_metrics"
	CollMetricsState       = "metrics_state"
)

// AllCollections lists every collection the store writes.
var AllCollections = []string{
	CollSessions, CollRuns, CollTeamSessions, CollWorkflowSessions, CollWorkflowV2Sessions,
	CollKnowledge, CollComponentMetrics, CollMetricsState,
}

// Backend is a keyed JSON document store.
//
// index holds string fields extracted from the document that List may filter
// on with equality (where). A backend may ignore where (the store re-checks
// every filter), but should push it down when it can.
type Backend interface {
	// Init prepares the backend (tables, indexes). It must be idempotent.
	Init(ctx context.Context) error
	Put(ctx context.Context, collection, id string, doc []byte, index map[string]string) error
	// Get returns (nil, nil) when the document does not exist.
	Get(ctx context.Context, collection, id string) ([]byte, error)
	Delete(ctx context.Context, collection, id string) error
	List(ctx context.Context, collection string, where map[string]string) ([][]byte, error)
	// Drop removes every document of a collection.
	Drop(ctx context.Context, collection string) error
}

// Inserter is optionally implemented by backends with an atomic
// insert-if-absent. It returns false when the id already exists.
type Inserter interface {
	Insert(ctx context.Context, collection, id string, doc []byte, index map[string]string) (bool, error)
}

// Store implements the storage contracts over a Backend.
type Store struct {
	*storage.MetricsEngine

	backend Backend
	id      string
	mu      sync.RWMutex
	mode    storage.StorageMode
}

// Option configures a Store.
type Option func(*Store)

// WithID sets the storage id returned by GetID.
func WithID(id string) Option { return func(s *Store) { s.id = id } }

// WithMode sets the initial storage.Storage mode (default agent).
func WithMode(m storage.StorageMode) Option { return func(s *Store) { s.mode = m } }

// New creates a Store over b.
func New(b Backend, opts ...Option) *Store {
	s := &Store{backend: b, id: "kvstore", mode: storage.AgentMode}
	for _, o := range opts {
		o(s)
	}
	s.MetricsEngine = storage.NewMetricsEngine(s)
	return s
}

// Backend returns the underlying backend.
func (s *Store) Backend() Backend { return s.backend }

func (s *Store) put(ctx context.Context, coll, id string, v interface{}, index map[string]string) error {
	doc, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("kvstore: encode %s/%s: %w", coll, id, err)
	}
	return s.backend.Put(ctx, coll, id, doc, index)
}

func (s *Store) get(ctx context.Context, coll, id string, v interface{}) (bool, error) {
	doc, err := s.backend.Get(ctx, coll, id)
	if err != nil || doc == nil {
		return false, err
	}
	if err := json.Unmarshal(doc, v); err != nil {
		return false, fmt.Errorf("kvstore: decode %s/%s: %w", coll, id, err)
	}
	return true, nil
}

func sessionIndex(sess *storage.AgentSession) map[string]string {
	return map[string]string{"user_id": sess.UserID, "agent_id": sess.AgentID}
}

func runIndex(r *storage.AgentRun) map[string]string {
	return map[string]string{"session_id": r.SessionID, "user_id": r.UserID}
}

func normalizeSession(sess *storage.AgentSession) {
	if sess.Memory == nil {
		sess.Memory = map[string]interface{}{}
	}
	if sess.SessionData == nil {
		sess.SessionData = map[string]interface{}{}
	}
	if sess.ExtraData == nil {
		sess.ExtraData = map[string]interface{}{}
	}
	if sess.AgentData == nil {
		sess.AgentData = map[string]interface{}{}
	}
}

// ---------------------------------------------------------------------------
// storage.DB
// ---------------------------------------------------------------------------

// CreateTables initializes the backend.
func (s *Store) CreateTables(ctx context.Context) error { return s.backend.Init(ctx) }

// UpgradeSchema is a no-op (schemaless).
func (s *Store) UpgradeSchema(_ context.Context) error { return nil }

// DropTables removes every collection.
func (s *Store) DropTables(ctx context.Context) error {
	for _, c := range AllCollections {
		if err := s.backend.Drop(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// CreateSession inserts a session, failing when it already exists.
func (s *Store) CreateSession(ctx context.Context, sess *storage.AgentSession) error {
	if sess == nil || sess.SessionID == "" {
		return fmt.Errorf("kvstore: session and session_id are required")
	}
	if ins, ok := s.backend.(Inserter); ok {
		doc, err := json.Marshal(sess)
		if err != nil {
			return err
		}
		created, err := ins.Insert(ctx, CollSessions, sess.SessionID, doc, sessionIndex(sess))
		if err != nil {
			return err
		}
		if !created {
			return fmt.Errorf("kvstore: session %q already exists", sess.SessionID)
		}
		return nil
	}
	existing, err := s.backend.Get(ctx, CollSessions, sess.SessionID)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("kvstore: session %q already exists", sess.SessionID)
	}
	return s.put(ctx, CollSessions, sess.SessionID, sess, sessionIndex(sess))
}

// ReadSession returns a session, or (nil, nil) when missing.
func (s *Store) ReadSession(ctx context.Context, sessionID string) (*storage.AgentSession, error) {
	var out storage.AgentSession
	ok, err := s.get(ctx, CollSessions, sessionID, &out)
	if err != nil || !ok {
		return nil, err
	}
	normalizeSession(&out)
	return &out, nil
}

// UpdateSession writes a session (upsert).
func (s *Store) UpdateSession(ctx context.Context, sess *storage.AgentSession) error {
	if sess == nil || sess.SessionID == "" {
		return fmt.Errorf("kvstore: session and session_id are required")
	}
	return s.put(ctx, CollSessions, sess.SessionID, sess, sessionIndex(sess))
}

// DeleteSession deletes a session of the current mode (agent sessions by default).
func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	return s.backend.Delete(ctx, s.modeCollection(), sessionID)
}

func (s *Store) listSessions(ctx context.Context, where map[string]string) ([]*storage.AgentSession, error) {
	docs, err := s.backend.List(ctx, CollSessions, where)
	if err != nil {
		return nil, err
	}
	out := make([]*storage.AgentSession, 0, len(docs))
	for _, d := range docs {
		var sess storage.AgentSession
		if err := json.Unmarshal(d, &sess); err != nil {
			return nil, fmt.Errorf("kvstore: decode session: %w", err)
		}
		if uid, ok := where["user_id"]; ok && sess.UserID != uid {
			continue
		}
		normalizeSession(&sess)
		out = append(out, &sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out, nil
}

func userWhere(userID string) map[string]string {
	if userID == "" {
		return nil
	}
	return map[string]string{"user_id": userID}
}

// GetAllSessionIDs returns the session ids of a user ("" = all), sorted.
func (s *Store) GetAllSessionIDs(ctx context.Context, userID string) ([]string, error) {
	sessions, err := s.listSessions(ctx, userWhere(userID))
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(sessions))
	for _, x := range sessions {
		ids = append(ids, x.SessionID)
	}
	return ids, nil
}

// GetAllSessions returns the sessions of a user ("" = all), sorted by id.
func (s *Store) GetAllSessions(ctx context.Context, userID string) ([]*storage.AgentSession, error) {
	return s.listSessions(ctx, userWhere(userID))
}

// CreateRun inserts a run, failing when it already exists.
func (s *Store) CreateRun(ctx context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("kvstore: run and id are required")
	}
	if ins, ok := s.backend.(Inserter); ok {
		doc, err := json.Marshal(run)
		if err != nil {
			return err
		}
		created, err := ins.Insert(ctx, CollRuns, run.ID, doc, runIndex(run))
		if err != nil {
			return err
		}
		if !created {
			return fmt.Errorf("kvstore: run %q already exists", run.ID)
		}
		return nil
	}
	existing, err := s.backend.Get(ctx, CollRuns, run.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("kvstore: run %q already exists", run.ID)
	}
	return s.put(ctx, CollRuns, run.ID, run, runIndex(run))
}

// ReadRun returns a run, or (nil, nil) when missing.
func (s *Store) ReadRun(ctx context.Context, runID string) (*storage.AgentRun, error) {
	var out storage.AgentRun
	ok, err := s.get(ctx, CollRuns, runID, &out)
	if err != nil || !ok {
		return nil, err
	}
	return &out, nil
}

// UpdateRun writes a run (upsert).
func (s *Store) UpdateRun(ctx context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("kvstore: run and id are required")
	}
	return s.put(ctx, CollRuns, run.ID, run, runIndex(run))
}

// DeleteRun deletes a run.
func (s *Store) DeleteRun(ctx context.Context, runID string) error {
	return s.backend.Delete(ctx, CollRuns, runID)
}

func (s *Store) listRuns(ctx context.Context, where map[string]string) ([]*storage.AgentRun, error) {
	docs, err := s.backend.List(ctx, CollRuns, where)
	if err != nil {
		return nil, err
	}
	out := make([]*storage.AgentRun, 0, len(docs))
	for _, d := range docs {
		var r storage.AgentRun
		if err := json.Unmarshal(d, &r); err != nil {
			return nil, fmt.Errorf("kvstore: decode run: %w", err)
		}
		if sid, ok := where["session_id"]; ok && r.SessionID != sid {
			continue
		}
		out = append(out, &r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// GetRunsForSession returns the runs of a session sorted by id.
func (s *Store) GetRunsForSession(ctx context.Context, sessionID string) ([]*storage.AgentRun, error) {
	return s.listRuns(ctx, map[string]string{"session_id": sessionID})
}

// ---------------------------------------------------------------------------
// storage.SessionSearcher
// ---------------------------------------------------------------------------

// SearchSessions filters, sorts and paginates sessions client-side.
func (s *Store) SearchSessions(ctx context.Context, filter *storage.SessionFilter, opts storage.SessionSearchOptions) ([]*storage.AgentSession, int, error) {
	var where map[string]string
	if opts.UserID != nil {
		where = map[string]string{"user_id": *opts.UserID}
	}
	sessions, err := s.listSessions(ctx, where)
	if err != nil {
		return nil, 0, err
	}
	return storage.FilterSortPaginate(sessions, filter, opts)
}

// ---------------------------------------------------------------------------
// storage.MetricsBackend (the MetricsStore API comes from the embedded engine)
// ---------------------------------------------------------------------------

// ListSessionsForMetrics lists the sessions passing f.
func (s *Store) ListSessionsForMetrics(ctx context.Context, f storage.MetricsSourceFilter) ([]*storage.AgentSession, error) {
	all, err := s.listSessions(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, x := range all {
		if f.MatchSession(x) {
			out = append(out, x)
		}
	}
	return out, nil
}

// ListRunsForMetrics lists the runs passing f.
func (s *Store) ListRunsForMetrics(ctx context.Context, f storage.MetricsSourceFilter) ([]*storage.AgentRun, error) {
	all, err := s.listRuns(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, x := range all {
		if f.MatchRun(x) {
			out = append(out, x)
		}
	}
	return out, nil
}

// ReplaceComponentMetrics replaces the component rows of a day.
func (s *Store) ReplaceComponentMetrics(ctx context.Context, day string, rows []storage.ComponentMetricsRow) error {
	existing, err := s.ListComponentMetrics(ctx, day, day)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, r := range rows {
		keep[r.ID] = true
		if err := s.put(ctx, CollComponentMetrics, r.ID, r, map[string]string{"date": r.Date}); err != nil {
			return err
		}
	}
	for _, r := range existing {
		if !keep[r.ID] {
			if err := s.backend.Delete(ctx, CollComponentMetrics, r.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListComponentMetrics lists component rows with startDay <= date <= endDay.
func (s *Store) ListComponentMetrics(ctx context.Context, startDay, endDay string) ([]storage.ComponentMetricsRow, error) {
	var where map[string]string
	if startDay != "" && startDay == endDay {
		where = map[string]string{"date": startDay}
	}
	docs, err := s.backend.List(ctx, CollComponentMetrics, where)
	if err != nil {
		return nil, err
	}
	var out []storage.ComponentMetricsRow
	for _, d := range docs {
		var r storage.ComponentMetricsRow
		if err := json.Unmarshal(d, &r); err != nil {
			return nil, fmt.Errorf("kvstore: decode metrics: %w", err)
		}
		if (startDay != "" && r.Date < startDay) || (endDay != "" && r.Date > endDay) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// GetMetricsState returns the metrics calculation state.
func (s *Store) GetMetricsState(ctx context.Context) (*storage.MetricsState, error) {
	var st storage.MetricsState
	ok, err := s.get(ctx, CollMetricsState, "metrics", &st)
	if err != nil || !ok {
		return nil, err
	}
	return &st, nil
}

// SetMetricsState stores the metrics calculation state.
func (s *Store) SetMetricsState(ctx context.Context, st *storage.MetricsState) error {
	return s.put(ctx, CollMetricsState, "metrics", st, nil)
}

// ---------------------------------------------------------------------------
// storage.KnowledgeStorage
// ---------------------------------------------------------------------------

// GetKnowledgeContent returns a knowledge row, or (nil, nil) when missing.
func (s *Store) GetKnowledgeContent(id string) (*storage.KnowledgeRow, error) {
	var out storage.KnowledgeRow
	ok, err := s.get(context.Background(), CollKnowledge, id, &out)
	if err != nil || !ok {
		return nil, err
	}
	return &out, nil
}

// GetKnowledgeContents returns a page of knowledge rows and the total count.
func (s *Store) GetKnowledgeContents(limit, page *int, sortBy, sortOrder *string) ([]*storage.KnowledgeRow, int, error) {
	docs, err := s.backend.List(context.Background(), CollKnowledge, nil)
	if err != nil {
		return nil, 0, err
	}
	rows := make([]*storage.KnowledgeRow, 0, len(docs))
	for _, d := range docs {
		var r storage.KnowledgeRow
		if err := json.Unmarshal(d, &r); err != nil {
			return nil, 0, fmt.Errorf("kvstore: decode knowledge: %w", err)
		}
		rows = append(rows, &r)
	}
	return storage.SortPaginateKnowledge(rows, limit, page, sortBy, sortOrder), len(rows), nil
}

// UpsertKnowledgeContent writes a knowledge row.
func (s *Store) UpsertKnowledgeContent(row *storage.KnowledgeRow) (*storage.KnowledgeRow, error) {
	if row == nil || row.ID == "" {
		return nil, fmt.Errorf("kvstore: knowledge row and id are required")
	}
	now := time.Now().Unix()
	if row.CreatedAt == nil {
		row.CreatedAt = &now
	}
	row.UpdatedAt = &now
	if err := s.put(context.Background(), CollKnowledge, row.ID, row, nil); err != nil {
		return nil, err
	}
	return row, nil
}

// DeleteKnowledgeContent deletes a knowledge row.
func (s *Store) DeleteKnowledgeContent(id string) error {
	return s.backend.Delete(context.Background(), CollKnowledge, id)
}

// ---------------------------------------------------------------------------
// storage.Storage (mode-based legacy contract used by teams)
// ---------------------------------------------------------------------------

// GetID returns the storage id.
func (s *Store) GetID() string { return s.id }

// GetMode returns the storage mode.
func (s *Store) GetMode() storage.StorageMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// SetMode sets the storage mode.
func (s *Store) SetMode(m storage.StorageMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mode = m
}

func (s *Store) modeCollection() string {
	switch s.GetMode() {
	case storage.TeamMode:
		return CollTeamSessions
	case storage.WorkflowMode:
		return CollWorkflowSessions
	case storage.WorkflowV2Mode:
		return CollWorkflowV2Sessions
	}
	return CollSessions
}

// Create initializes the backend.
func (s *Store) Create() error { return s.backend.Init(context.Background()) }

// Drop removes every collection.
func (s *Store) Drop() error { return s.DropTables(context.Background()) }

// TableExists always reports true: collections are created lazily.
func (s *Store) TableExists() (bool, error) { return true, nil }

func (s *Store) decodeModeSession(doc []byte) (interface{}, string, string, error) {
	switch s.GetMode() {
	case storage.TeamMode:
		var t storage.TeamSession
		err := json.Unmarshal(doc, &t)
		return &t, t.UserID, t.TeamID, err
	case storage.WorkflowMode:
		var w storage.WorkflowSession
		err := json.Unmarshal(doc, &w)
		return &w, w.UserID, w.WorkflowID, err
	case storage.WorkflowV2Mode:
		var w storage.WorkflowSessionV2
		err := json.Unmarshal(doc, &w)
		return &w, w.UserID, w.WorkflowID, err
	}
	var a storage.AgentSession
	err := json.Unmarshal(doc, &a)
	normalizeSession(&a)
	return &a, a.UserID, a.AgentID, err
}

// Read reads a session of the current mode, optionally scoped to a user.
func (s *Store) Read(sessionID string, userID *string) (interface{}, error) {
	doc, err := s.backend.Get(context.Background(), s.modeCollection(), sessionID)
	if err != nil || doc == nil {
		return nil, err
	}
	sess, uid, _, err := s.decodeModeSession(doc)
	if err != nil {
		return nil, fmt.Errorf("kvstore: decode session: %w", err)
	}
	if userID != nil && uid != *userID {
		return nil, nil
	}
	return sess, nil
}

// Upsert writes an agent, team or workflow session (pointer or value).
func (s *Store) Upsert(session interface{}) (interface{}, error) {
	ctx := context.Background()
	switch v := session.(type) {
	case storage.AgentSession:
		return s.Upsert(&v)
	case storage.TeamSession:
		return s.Upsert(&v)
	case storage.WorkflowSession:
		return s.Upsert(&v)
	case storage.WorkflowSessionV2:
		return s.Upsert(&v)
	case *storage.AgentSession:
		v.UpdatedAt = time.Now().Unix()
		return v, s.put(ctx, CollSessions, v.SessionID, v, sessionIndex(v))
	case *storage.TeamSession:
		v.UpdatedAt = time.Now().Unix()
		return v, s.put(ctx, CollTeamSessions, v.SessionID, v, map[string]string{"user_id": v.UserID, "entity_id": v.TeamID})
	case *storage.WorkflowSession:
		v.UpdatedAt = time.Now().Unix()
		return v, s.put(ctx, CollWorkflowSessions, v.SessionID, v, map[string]string{"user_id": v.UserID, "entity_id": v.WorkflowID})
	case *storage.WorkflowSessionV2:
		v.UpdatedAt = time.Now().Unix()
		return v, s.put(ctx, CollWorkflowV2Sessions, v.SessionID, v, map[string]string{"user_id": v.UserID, "entity_id": v.WorkflowID})
	}
	return nil, fmt.Errorf("kvstore: unsupported session type %T", session)
}

// GetRecentSessions returns the most recent sessions of the current mode.
func (s *Store) GetRecentSessions(userID *string, entityID *string, limit *int) ([]interface{}, error) {
	where := map[string]string{}
	if userID != nil {
		where["user_id"] = *userID
	}
	docs, err := s.backend.List(context.Background(), s.modeCollection(), where)
	if err != nil {
		return nil, err
	}
	type item struct {
		v       interface{}
		created int64
		id      string
	}
	var items []item
	for _, d := range docs {
		sess, uid, eid, err := s.decodeModeSession(d)
		if err != nil {
			return nil, fmt.Errorf("kvstore: decode session: %w", err)
		}
		if (userID != nil && uid != *userID) || (entityID != nil && eid != *entityID) {
			continue
		}
		var meta struct {
			SessionID string `json:"session_id"`
			CreatedAt int64  `json:"created_at"`
		}
		_ = json.Unmarshal(d, &meta)
		items = append(items, item{sess, meta.CreatedAt, meta.SessionID})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].created == items[j].created {
			return items[i].id > items[j].id
		}
		return items[i].created > items[j].created
	})
	if limit != nil && *limit > 0 && len(items) > *limit {
		items = items[:*limit]
	}
	out := make([]interface{}, len(items))
	for i, it := range items {
		out[i] = it.v
	}
	return out, nil
}

// Interface assertions.
var (
	_ storage.DB               = (*Store)(nil)
	_ storage.Storage          = (*Store)(nil)
	_ storage.KnowledgeStorage = (*Store)(nil)
	_ storage.SessionSearcher  = (*Store)(nil)
	_ storage.MetricsStore     = (*Store)(nil)
	_ storage.MetricsBackend   = (*Store)(nil)
)

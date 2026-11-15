package component

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// SessionInfo summarizes a stored session.
type SessionInfo struct {
	SessionID   string         `json:"session_id"`
	SessionName string         `json:"session_name,omitempty"`
	SessionType Kind           `json:"session_type,omitempty"`
	ComponentID string         `json:"component_id,omitempty"`
	UserID      string         `json:"user_id,omitempty"`
	CreatedAt   int64          `json:"created_at,omitempty"`
	UpdatedAt   int64          `json:"updated_at,omitempty"`
	State       map[string]any `json:"session_state,omitempty"`
}

// RunInfo is one run of a session's history.
type RunInfo struct {
	RunID     string         `json:"run_id"`
	SessionID string         `json:"session_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Input     string         `json:"run_input,omitempty"`
	Content   string         `json:"content,omitempty"`
	CreatedAt int64          `json:"created_at,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// SessionQuery filters ListSessions. Limit/Page are one-based pagination.
type SessionQuery struct {
	SessionType Kind
	ComponentID string
	UserID      string
	SessionName string
	DBID        string
	Limit       int
	Page        int
	SortBy      string // created_at | updated_at
	SortOrder   string // asc | desc
}

// SessionStore is the read-only session surface the protocol interfaces use
// (Python's get_sessions / get_session_runs MCP tools).
type SessionStore interface {
	ListSessions(ctx context.Context, q SessionQuery) ([]SessionInfo, int, error)
	GetSessionRuns(ctx context.Context, sessionID string, sessionType Kind, userID, dbID string) ([]RunInfo, error)
	DatabaseIDs() []string
}

// StorageSessionStore implements SessionStore over storage.DB instances,
// keyed by database id. Sessions are attributed to agents (storage.DB only
// stores agent sessions).
type StorageSessionStore struct {
	DBs map[string]storage.DB
}

// NewStorageSessionStore returns a store over the given databases.
func NewStorageSessionStore(dbs map[string]storage.DB) *StorageSessionStore {
	return &StorageSessionStore{DBs: dbs}
}

func (s *StorageSessionStore) DatabaseIDs() []string {
	ids := make([]string, 0, len(s.DBs))
	for id := range s.DBs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s *StorageSessionStore) db(dbID string) (storage.DB, error) {
	if dbID != "" {
		db, ok := s.DBs[dbID]
		if !ok {
			return nil, fmt.Errorf("database %q %w", dbID, ErrNotFound)
		}
		return db, nil
	}
	switch len(s.DBs) {
	case 0:
		return nil, fmt.Errorf("no session database configured")
	case 1:
		for _, db := range s.DBs {
			return db, nil
		}
	}
	return nil, fmt.Errorf("multiple databases configured: db_id is required (one of %s)", strings.Join(s.DatabaseIDs(), ", "))
}

func (s *StorageSessionStore) ListSessions(ctx context.Context, q SessionQuery) ([]SessionInfo, int, error) {
	if q.SessionType != "" && q.SessionType != KindAgent {
		return []SessionInfo{}, 0, nil
	}
	db, err := s.db(q.DBID)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := db.GetAllSessions(ctx, q.UserID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		if sess == nil || (q.ComponentID != "" && sess.AgentID != q.ComponentID) {
			continue
		}
		name, _ := sess.SessionData["session_name"].(string)
		if q.SessionName != "" && name != q.SessionName {
			continue
		}
		state, _ := sess.SessionData["session_state"].(map[string]any)
		out = append(out, SessionInfo{
			SessionID: sess.SessionID, SessionName: name, SessionType: KindAgent,
			ComponentID: sess.AgentID, UserID: sess.UserID,
			CreatedAt: sess.CreatedAt, UpdatedAt: sess.UpdatedAt, State: state,
		})
	}
	return Paginate(out, q)
}

func (s *StorageSessionStore) GetSessionRuns(ctx context.Context, sessionID string, _ Kind, userID, dbID string) ([]RunInfo, error) {
	db, err := s.db(dbID)
	if err != nil {
		return nil, err
	}
	if userID != "" {
		sess, err := db.ReadSession(ctx, sessionID)
		if err != nil || sess == nil {
			return nil, fmt.Errorf("session %s %w", sessionID, ErrNotFound)
		}
		if sess.UserID != "" && sess.UserID != userID {
			return nil, fmt.Errorf("session %s %w", sessionID, ErrNotFound)
		}
	}
	runs, err := db.GetRunsForSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]RunInfo, 0, len(runs))
	for _, r := range runs {
		if r == nil {
			continue
		}
		status, _ := r.RunData["status"].(string)
		out = append(out, RunInfo{
			RunID: r.ID, SessionID: r.SessionID, Status: status,
			Input: r.UserMessage, Content: r.AgentMessage, CreatedAt: r.CreatedAt.Unix(),
			Details: map[string]any{"run_name": r.RunName, "run_data": r.RunData, "metrics": r.Metrics, "messages": r.Messages},
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

// Paginate sorts and pages sessions per q, returning the page and the total.
func Paginate(sessions []SessionInfo, q SessionQuery) ([]SessionInfo, int, error) {
	key := func(s SessionInfo) int64 {
		if q.SortBy == "updated_at" {
			return s.UpdatedAt
		}
		return s.CreatedAt
	}
	asc := strings.EqualFold(q.SortOrder, "asc")
	sort.SliceStable(sessions, func(i, j int) bool {
		if asc {
			return key(sessions[i]) < key(sessions[j])
		}
		return key(sessions[i]) > key(sessions[j])
	})
	total := len(sessions)
	limit, page := q.Limit, q.Page
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= total {
		return []SessionInfo{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return sessions[start:end], total, nil
}

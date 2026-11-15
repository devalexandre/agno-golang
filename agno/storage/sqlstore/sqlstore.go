// Package sqlstore implements storage.DB on top of database/sql, shared by the
// postgres and mysql backends. JSON-typed fields are stored as TEXT columns and
// timestamps as BIGINT unix seconds, keeping the schema portable across engines.
package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// Dialect captures the small differences between SQL engines.
type Dialect struct {
	// Driver is the database/sql driver name ("postgres" or "mysql").
	Driver string
	// Positional reports whether placeholders are positional ($1) vs "?".
	Positional bool
	// CaseInsensitiveCollation reports that the default string collation is
	// case-insensitive (MySQL, SingleStore); session filters then compare
	// with BINARY to keep EQ/IN case-sensitive.
	CaseInsensitiveCollation bool
}

// ph renders placeholder n (1-based) for the dialect.
func (d Dialect) ph(n int) string {
	if d.Positional {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// rebind converts a "?"-style query into the dialect's placeholder style.
func (d Dialect) rebind(query string) string {
	if !d.Positional {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString(fmt.Sprintf("$%d", n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SQLStore is a database/sql-backed storage.DB.
type SQLStore struct {
	// MetricsEngine provides the storage.MetricsStore API over this store.
	*storage.MetricsEngine

	db        *sql.DB
	dialect   Dialect
	prefix    string
	sessTable string
	runTable  string
}

// New creates a SQLStore from an existing *sql.DB.
func New(db *sql.DB, dialect Dialect, tablePrefix string) *SQLStore {
	if tablePrefix == "" {
		tablePrefix = "agno"
	}
	s := &SQLStore{
		db:        db,
		dialect:   dialect,
		prefix:    tablePrefix,
		sessTable: tablePrefix + "_sessions",
		runTable:  tablePrefix + "_runs",
	}
	s.MetricsEngine = storage.NewMetricsEngine(s)
	return s
}

// DB returns the underlying *sql.DB.
func (s *SQLStore) DB() *sql.DB { return s.db }

func mustJSON(v interface{}) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseMap(raw string) map[string]interface{} {
	m := make(map[string]interface{})
	if raw == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

func parseMessages(raw string) []map[string]interface{} {
	var out []map[string]interface{}
	if raw == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// CreateTables creates the session and run tables if they do not exist.
func (s *SQLStore) CreateTables(ctx context.Context) error {
	sessDDL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		session_id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255),
		agent_id VARCHAR(255),
		memory TEXT,
		session_data TEXT,
		extra_data TEXT,
		agent_data TEXT,
		team_session_id VARCHAR(255),
		created_at BIGINT,
		updated_at BIGINT
	)`, s.sessTable)

	runDDL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id VARCHAR(255) PRIMARY KEY,
		session_id VARCHAR(255),
		user_id VARCHAR(255),
		run_name VARCHAR(255),
		run_data TEXT,
		user_message TEXT,
		agent_message TEXT,
		messages TEXT,
		metrics TEXT,
		created_at BIGINT,
		updated_at BIGINT
	)`, s.runTable)

	if _, err := s.db.ExecContext(ctx, sessDDL); err != nil {
		return fmt.Errorf("sqlstore: create sessions table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, runDDL); err != nil {
		return fmt.Errorf("sqlstore: create runs table: %w", err)
	}
	return s.createExtendedTables(ctx)
}

// UpgradeSchema is a no-op (single schema version).
func (s *SQLStore) UpgradeSchema(_ context.Context) error { return nil }

// DropTables drops the session and run tables.
func (s *SQLStore) DropTables(ctx context.Context) error {
	for _, t := range append([]string{s.sessTable, s.runTable}, s.extendedTables()...) {
		if _, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+t); err != nil {
			return fmt.Errorf("sqlstore: drop %s: %w", t, err)
		}
	}
	return nil
}

// CreateSession inserts a new session.
func (s *SQLStore) CreateSession(ctx context.Context, sess *storage.AgentSession) error {
	if sess == nil || sess.SessionID == "" {
		return fmt.Errorf("sqlstore: session and session_id are required")
	}
	q := s.dialect.rebind(fmt.Sprintf(
		`INSERT INTO %s (session_id, user_id, agent_id, memory, session_data, extra_data, agent_data, team_session_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.sessTable))
	_, err := s.db.ExecContext(ctx, q,
		sess.SessionID, sess.UserID, sess.AgentID,
		mustJSON(sess.Memory), mustJSON(sess.SessionData), mustJSON(sess.ExtraData), mustJSON(sess.AgentData),
		teamSessionID(sess), sess.CreatedAt, sess.UpdatedAt,
	)
	return err
}

func teamSessionID(s *storage.AgentSession) interface{} {
	if s.TeamSessionID != nil {
		return *s.TeamSessionID
	}
	return nil
}

// ReadSession returns a session by ID, or (nil, nil) when not found.
func (s *SQLStore) ReadSession(ctx context.Context, sessionID string) (*storage.AgentSession, error) {
	q := s.dialect.rebind(fmt.Sprintf(
		`SELECT session_id, user_id, agent_id, memory, session_data, extra_data, agent_data, team_session_id, created_at, updated_at
		 FROM %s WHERE session_id = ?`, s.sessTable))

	var (
		out                                    storage.AgentSession
		memory, sessData, extraData, agentData string
		teamSess                               sql.NullString
	)
	err := s.db.QueryRowContext(ctx, q, sessionID).Scan(
		&out.SessionID, &out.UserID, &out.AgentID,
		&memory, &sessData, &extraData, &agentData,
		&teamSess, &out.CreatedAt, &out.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out.Memory = parseMap(memory)
	out.SessionData = parseMap(sessData)
	out.ExtraData = parseMap(extraData)
	out.AgentData = parseMap(agentData)
	if teamSess.Valid {
		v := teamSess.String
		out.TeamSessionID = &v
	}
	return &out, nil
}

// UpdateSession updates an existing session.
func (s *SQLStore) UpdateSession(ctx context.Context, sess *storage.AgentSession) error {
	if sess == nil || sess.SessionID == "" {
		return fmt.Errorf("sqlstore: session and session_id are required")
	}
	q := s.dialect.rebind(fmt.Sprintf(
		`UPDATE %s SET user_id=?, agent_id=?, memory=?, session_data=?, extra_data=?, agent_data=?, team_session_id=?, updated_at=?
		 WHERE session_id=?`, s.sessTable))
	_, err := s.db.ExecContext(ctx, q,
		sess.UserID, sess.AgentID,
		mustJSON(sess.Memory), mustJSON(sess.SessionData), mustJSON(sess.ExtraData), mustJSON(sess.AgentData),
		teamSessionID(sess), sess.UpdatedAt, sess.SessionID,
	)
	return err
}

// DeleteSession deletes a session by ID.
func (s *SQLStore) DeleteSession(ctx context.Context, sessionID string) error {
	q := s.dialect.rebind(fmt.Sprintf(`DELETE FROM %s WHERE session_id = ?`, s.sessTable))
	_, err := s.db.ExecContext(ctx, q, sessionID)
	return err
}

// GetAllSessionIDs returns all session IDs for a user.
func (s *SQLStore) GetAllSessionIDs(ctx context.Context, userID string) ([]string, error) {
	q := fmt.Sprintf(`SELECT session_id FROM %s`, s.sessTable)
	args := []interface{}{}
	if userID != "" {
		q = s.dialect.rebind(q + ` WHERE user_id = ?`)
		args = append(args, userID)
	}
	q += ` ORDER BY session_id`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAllSessions returns all sessions for a user.
func (s *SQLStore) GetAllSessions(ctx context.Context, userID string) ([]*storage.AgentSession, error) {
	ids, err := s.GetAllSessionIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []*storage.AgentSession
	for _, id := range ids {
		sess, err := s.ReadSession(ctx, id)
		if err != nil {
			return nil, err
		}
		if sess != nil {
			out = append(out, sess)
		}
	}
	return out, nil
}

// CreateRun inserts a new run.
func (s *SQLStore) CreateRun(ctx context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("sqlstore: run and id are required")
	}
	q := s.dialect.rebind(fmt.Sprintf(
		`INSERT INTO %s (id, session_id, user_id, run_name, run_data, user_message, agent_message, messages, metrics, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.runTable))
	_, err := s.db.ExecContext(ctx, q,
		run.ID, run.SessionID, run.UserID, run.RunName,
		mustJSON(run.RunData), run.UserMessage, run.AgentMessage,
		mustJSON(run.Messages), mustJSON(run.Metrics),
		run.CreatedAt.Unix(), run.UpdatedAt.Unix(),
	)
	return err
}

// ReadRun returns a run by ID, or (nil, nil) when not found.
func (s *SQLStore) ReadRun(ctx context.Context, runID string) (*storage.AgentRun, error) {
	q := s.dialect.rebind(fmt.Sprintf(
		`SELECT id, session_id, user_id, run_name, run_data, user_message, agent_message, messages, metrics, created_at, updated_at
		 FROM %s WHERE id = ?`, s.runTable))

	var (
		out                        storage.AgentRun
		runData, messages, metrics string
		createdAt, updatedAt       int64
	)
	err := s.db.QueryRowContext(ctx, q, runID).Scan(
		&out.ID, &out.SessionID, &out.UserID, &out.RunName,
		&runData, &out.UserMessage, &out.AgentMessage,
		&messages, &metrics, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out.RunData = parseMap(runData)
	out.Messages = parseMessages(messages)
	out.Metrics = parseMap(metrics)
	out.CreatedAt = time.Unix(createdAt, 0)
	out.UpdatedAt = time.Unix(updatedAt, 0)
	return &out, nil
}

// UpdateRun updates an existing run.
func (s *SQLStore) UpdateRun(ctx context.Context, run *storage.AgentRun) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("sqlstore: run and id are required")
	}
	q := s.dialect.rebind(fmt.Sprintf(
		`UPDATE %s SET session_id=?, user_id=?, run_name=?, run_data=?, user_message=?, agent_message=?, messages=?, metrics=?, updated_at=?
		 WHERE id=?`, s.runTable))
	_, err := s.db.ExecContext(ctx, q,
		run.SessionID, run.UserID, run.RunName,
		mustJSON(run.RunData), run.UserMessage, run.AgentMessage,
		mustJSON(run.Messages), mustJSON(run.Metrics), run.UpdatedAt.Unix(), run.ID,
	)
	return err
}

// DeleteRun deletes a run by ID.
func (s *SQLStore) DeleteRun(ctx context.Context, runID string) error {
	q := s.dialect.rebind(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, s.runTable))
	_, err := s.db.ExecContext(ctx, q, runID)
	return err
}

// GetRunsForSession returns all runs for a session ordered by id.
func (s *SQLStore) GetRunsForSession(ctx context.Context, sessionID string) ([]*storage.AgentRun, error) {
	q := s.dialect.rebind(fmt.Sprintf(`SELECT id FROM %s WHERE session_id = ? ORDER BY id`, s.runTable))
	rows, err := s.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []*storage.AgentRun
	for _, id := range ids {
		r, err := s.ReadRun(ctx, id)
		if err != nil {
			return nil, err
		}
		if r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// Ensure SQLStore satisfies the storage.DB interface.
var _ storage.DB = (*SQLStore)(nil)

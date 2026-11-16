// Package sqlapproval persists approval records in a SQL table, with the same
// columns as Python's approvals table (JSON fields stored as TEXT, timestamps
// as BIGINT unix seconds). It is shared by the database/sql storage backends
// (postgres, mysql, singlestore, sqlite).
package sqlapproval

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
)

// Store is an approval.Store over a SQL table. The table is created on first
// use (CREATE TABLE IF NOT EXISTS), or explicitly with Init.
type Store struct {
	db         *sql.DB
	table      string
	positional bool // $1-style placeholders (postgres) instead of "?"

	initMu sync.Mutex
	ready  bool
}

// New creates a Store on table. positional selects $n placeholders (postgres).
func New(db *sql.DB, table string, positional bool) *Store {
	if table == "" {
		table = "agno_approvals"
	}
	return &Store{db: db, table: table, positional: positional}
}

// Table returns the table name.
func (s *Store) Table() string { return s.table }

const columns = `id, run_id, session_id, status, source_type, approval_type, pause_type, tool_name, tool_args,
	agent_id, team_id, workflow_id, user_id, source_name, requirements, context, resolution_data,
	resolved_by, resolved_at, run_status, created_at, updated_at`

// Init creates the table and its indexes if they do not exist.
func (s *Store) Init(ctx context.Context) error {
	s.initMu.Lock()
	defer s.initMu.Unlock()
	if s.ready {
		return nil
	}
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id VARCHAR(255) PRIMARY KEY,
		run_id VARCHAR(255) NOT NULL,
		session_id VARCHAR(255) NOT NULL,
		status VARCHAR(32) NOT NULL,
		source_type VARCHAR(32) NOT NULL,
		approval_type VARCHAR(32),
		pause_type VARCHAR(64) NOT NULL,
		tool_name VARCHAR(255),
		tool_args TEXT,
		agent_id VARCHAR(255),
		team_id VARCHAR(255),
		workflow_id VARCHAR(255),
		user_id VARCHAR(255),
		source_name VARCHAR(255),
		requirements TEXT,
		context TEXT,
		resolution_data TEXT,
		resolved_by VARCHAR(255),
		resolved_at BIGINT,
		run_status VARCHAR(32),
		created_at BIGINT NOT NULL,
		updated_at BIGINT
	)`, s.table)
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("sqlapproval: create table %s: %w", s.table, err)
	}
	s.ready = true
	return nil
}

// Drop drops the table.
func (s *Store) Drop(ctx context.Context) error {
	s.initMu.Lock()
	defer s.initMu.Unlock()
	s.ready = false
	_, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+s.table)
	return err
}

func (s *Store) rebind(q string) string {
	if !s.positional {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func toJSON(v interface{}) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// CreateApproval inserts a record.
func (s *Store) CreateApproval(ctx context.Context, a *approval.Approval) error {
	if a == nil || a.ID == "" {
		return errors.New("approval: id is required")
	}
	if err := s.Init(ctx); err != nil {
		return err
	}
	now := time.Now().Unix()
	if a.CreatedAt == 0 {
		a.CreatedAt = now
	}
	if a.UpdatedAt == 0 {
		a.UpdatedAt = a.CreatedAt
	}
	sourceType := a.SourceType
	if sourceType == "" {
		sourceType = "agent"
	}
	q := s.rebind(fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, s.table, columns))
	_, err := s.db.ExecContext(ctx, q,
		a.ID, a.RunID, a.SessionID, string(a.Status), sourceType, nullStr(string(a.ApprovalType)), string(a.PauseType),
		nullStr(a.ToolName), toJSON(a.ToolArgs),
		nullStr(a.AgentID), nullStr(a.TeamID), nullStr(a.WorkflowID), nullStr(a.UserID), nullStr(a.SourceName),
		toJSON(a.Requirements), toJSON(a.Context), toJSON(a.ResolutionData),
		nullStr(a.ResolvedBy), sql.NullInt64{Int64: a.ResolvedAt, Valid: a.ResolvedAt != 0}, nullStr(string(a.RunStatus)),
		a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlapproval: insert: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanApproval(r scanner) (*approval.Approval, error) {
	var (
		a                                              approval.Approval
		status, sourceType, pauseType                  string
		approvalType, toolName, agentID, teamID        sql.NullString
		workflowID, userID, sourceName, resolvedBy     sql.NullString
		toolArgs, reqs, ctxJSON, resolution, runStatus sql.NullString
		resolvedAt, updatedAt                          sql.NullInt64
	)
	if err := r.Scan(&a.ID, &a.RunID, &a.SessionID, &status, &sourceType, &approvalType, &pauseType, &toolName, &toolArgs,
		&agentID, &teamID, &workflowID, &userID, &sourceName, &reqs, &ctxJSON, &resolution,
		&resolvedBy, &resolvedAt, &runStatus, &a.CreatedAt, &updatedAt); err != nil {
		return nil, err
	}
	a.Status, a.SourceType, a.PauseType = approval.Status(status), sourceType, models.PauseType(pauseType)
	a.ApprovalType, a.ToolName = approval.Type(approvalType.String), toolName.String
	a.AgentID, a.TeamID, a.WorkflowID, a.UserID = agentID.String, teamID.String, workflowID.String, userID.String
	a.SourceName, a.ResolvedBy, a.RunStatus = sourceName.String, resolvedBy.String, models.RunStatus(runStatus.String)
	a.ResolvedAt, a.UpdatedAt = resolvedAt.Int64, updatedAt.Int64
	for _, f := range []struct {
		raw sql.NullString
		dst interface{}
	}{{toolArgs, &a.ToolArgs}, {reqs, &a.Requirements}, {ctxJSON, &a.Context}, {resolution, &a.ResolutionData}} {
		if f.raw.Valid && f.raw.String != "" {
			if err := json.Unmarshal([]byte(f.raw.String), f.dst); err != nil {
				return nil, fmt.Errorf("sqlapproval: decode approval %s: %w", a.ID, err)
			}
		}
	}
	return &a, nil
}

// GetApproval returns a record by id (approval.ErrNotFound when missing).
func (s *Store) GetApproval(ctx context.Context, id string) (*approval.Approval, error) {
	if err := s.Init(ctx); err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, s.rebind(fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, columns, s.table)), id)
	a, err := scanApproval(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, approval.ErrNotFound
	}
	return a, err
}

// ListApprovals returns matching records, most recent first. Errors are
// returned, never an empty page: the continue-run approval gate must not read
// a failed query as "no pending approval".
func (s *Store) ListApprovals(ctx context.Context, f approval.Filter) ([]*approval.Approval, error) {
	if err := s.Init(ctx); err != nil {
		return nil, err
	}
	var where []string
	var args []interface{}
	add := func(col, v string) {
		if v != "" {
			where = append(where, col+" = ?")
			args = append(args, v)
		}
	}
	add("run_id", f.RunID)
	add("session_id", f.SessionID)
	add("status", string(f.Status))
	add("approval_type", string(f.ApprovalType))
	q := fmt.Sprintf(`SELECT %s FROM %s`, columns, s.table)
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY created_at DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, fmt.Errorf("sqlapproval: list: %w", err)
	}
	defer rows.Close()
	var out []*approval.Approval
	for rows.Next() {
		a, err := scanApproval(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateApproval resolves a record when its status equals expected. The
// status check is part of the UPDATE, so concurrent resolutions cannot both win.
func (s *Store) UpdateApproval(ctx context.Context, id string, expected approval.Status, res approval.Resolution) (*approval.Approval, error) {
	if err := s.Init(ctx); err != nil {
		return nil, err
	}
	set := []string{"status = ?", "resolved_by = ?", "resolved_at = ?", "updated_at = ?"}
	args := []interface{}{string(res.Status), nullStr(res.ResolvedBy), sql.NullInt64{Int64: res.ResolvedAt, Valid: res.ResolvedAt != 0}, time.Now().Unix()}
	if res.ResolutionData != nil {
		set = append(set, "resolution_data = ?")
		args = append(args, toJSON(res.ResolutionData))
	}
	q := fmt.Sprintf(`UPDATE %s SET %s WHERE id = ?`, s.table, strings.Join(set, ", "))
	args = append(args, id)
	if expected != "" {
		q += " AND status = ?"
		args = append(args, string(expected))
	}
	result, err := s.db.ExecContext(ctx, s.rebind(q), args...)
	if err != nil {
		return nil, fmt.Errorf("sqlapproval: update: %w", err)
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		if _, gerr := s.GetApproval(ctx, id); gerr != nil {
			return nil, gerr
		}
		return nil, approval.ErrStatusConflict
	}
	return s.GetApproval(ctx, id)
}

// UpdateRunStatus stamps run_status on every approval of the run.
func (s *Store) UpdateRunStatus(ctx context.Context, runID string, status models.RunStatus) (int, error) {
	if err := s.Init(ctx); err != nil {
		return 0, err
	}
	q := fmt.Sprintf(`UPDATE %s SET run_status = ?, updated_at = ? WHERE run_id = ? AND (run_status IS NULL OR run_status <> ?)`, s.table)
	result, err := s.db.ExecContext(ctx, s.rebind(q), string(status), time.Now().Unix(), runID, string(status))
	if err != nil {
		return 0, fmt.Errorf("sqlapproval: update run status: %w", err)
	}
	n, err := result.RowsAffected()
	return int(n), err
}

// DeleteApproval removes a record. It reports whether one was deleted.
func (s *Store) DeleteApproval(ctx context.Context, id string) (bool, error) {
	if err := s.Init(ctx); err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, s.rebind(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, s.table)), id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n > 0, err
}

var _ approval.Store = (*Store)(nil)

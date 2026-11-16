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

// Extended tables: component metrics, metrics state and knowledge contents.

func (s *SQLStore) metricsTable() string   { return s.prefix + "_component_metrics" }
func (s *SQLStore) stateTable() string     { return s.prefix + "_metrics_state" }
func (s *SQLStore) knowledgeTable() string { return s.prefix + "_knowledge" }

func (s *SQLStore) extendedTables() []string {
	return []string{s.metricsTable(), s.stateTable(), s.knowledgeTable()}
}

func (s *SQLStore) createExtendedTables(ctx context.Context) error {
	ddls := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(512) PRIMARY KEY,
			metric_date VARCHAR(10),
			user_id VARCHAR(255),
			data TEXT,
			updated_at BIGINT
		)`, s.metricsTable()),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(64) PRIMARY KEY,
			data TEXT
		)`, s.stateTable()),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(512),
			data TEXT,
			created_at BIGINT,
			updated_at BIGINT
		)`, s.knowledgeTable()),
	}
	for _, ddl := range ddls {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("sqlstore: create extended table: %w", err)
		}
	}
	return nil
}

const sessionColumns = `session_id, user_id, agent_id, memory, session_data, extra_data, agent_data, team_session_id, created_at, updated_at`

func scanSessions(rows *sql.Rows) ([]*storage.AgentSession, error) {
	defer rows.Close()
	var out []*storage.AgentSession
	for rows.Next() {
		var (
			sess                                   storage.AgentSession
			userID, agentID                        sql.NullString
			memory, sessData, extraData, agentData sql.NullString
			teamSess                               sql.NullString
			createdAt, updatedAt                   sql.NullInt64
		)
		if err := rows.Scan(&sess.SessionID, &userID, &agentID, &memory, &sessData, &extraData, &agentData,
			&teamSess, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		sess.UserID, sess.AgentID = userID.String, agentID.String
		sess.Memory = parseMap(memory.String)
		sess.SessionData = parseMap(sessData.String)
		sess.ExtraData = parseMap(extraData.String)
		sess.AgentData = parseMap(agentData.String)
		if teamSess.Valid {
			v := teamSess.String
			sess.TeamSessionID = &v
		}
		sess.CreatedAt, sess.UpdatedAt = createdAt.Int64, updatedAt.Int64
		out = append(out, &sess)
	}
	return out, rows.Err()
}

func (s *SQLStore) querySessions(ctx context.Context, where string, args []interface{}, suffix string) ([]*storage.AgentSession, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE %s %s`, sessionColumns, s.sessTable, where, suffix)
	rows, err := s.db.QueryContext(ctx, s.dialect.rebind(q), args...)
	if err != nil {
		return nil, err
	}
	return scanSessions(rows)
}

// sessionFilterColumns are the filter keys backed by native columns.
var sessionFilterColumns = map[string]string{
	"session_id": "session_id",
	"user_id":    "user_id",
	"agent_id":   "agent_id",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

// SearchSessions implements storage.SessionSearcher. Filters on native
// columns (session_id, user_id, agent_id, created_at, updated_at) run in SQL
// with pagination pushed down; filters on fields kept inside JSON columns
// (session_name, team_id, workflow_id, session_type) fall back to the
// portable in-memory evaluation over the user's sessions.
func (s *SQLStore) SearchSessions(ctx context.Context, filter *storage.SessionFilter, opts storage.SessionSearchOptions) ([]*storage.AgentSession, int, error) {
	opts, err := opts.Normalize()
	if err != nil {
		return nil, 0, err
	}
	nf, err := storage.NormalizeSessionFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	clause, args, ok := storage.SessionFilterToSQL(nf, storage.SessionFilterSQLOptions{
		Columns:       sessionFilterColumns,
		BinaryCompare: s.dialect.CaseInsensitiveCollation,
		NullIfEmpty:   true,
	})
	if opts.UserID != nil {
		clause = "(" + clause + ") AND user_id = ?"
		if !ok {
			clause = "user_id = ?"
		}
		args = append(args, *opts.UserID)
	} else if !ok {
		clause, args = "1=1", nil
	}
	if !ok {
		all, err := s.querySessions(ctx, clause, args, "")
		if err != nil {
			return nil, 0, err
		}
		return storage.FilterSortPaginate(all, nf, opts)
	}

	var total int
	countQ := s.dialect.rebind(fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, s.sessTable, clause))
	if err := s.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := strings.ToUpper(opts.SortOrder)
	suffix := fmt.Sprintf("ORDER BY %s %s, session_id %s LIMIT %d OFFSET %d",
		opts.SortBy, order, order, opts.Limit, (opts.Page-1)*opts.Limit)
	page, err := s.querySessions(ctx, clause, args, suffix)
	if err != nil {
		return nil, 0, err
	}
	if page == nil {
		page = []*storage.AgentSession{}
	}
	return page, total, nil
}

// ---------------------------------------------------------------------------
// storage.MetricsBackend
// ---------------------------------------------------------------------------

func sourceWhere(f storage.MetricsSourceFilter) (string, []interface{}) {
	conds := []string{"1=1"}
	var args []interface{}
	if f.CreatedFrom != 0 {
		conds = append(conds, "created_at >= ?")
		args = append(args, f.CreatedFrom)
	}
	if f.CreatedTo != 0 {
		conds = append(conds, "created_at < ?")
		args = append(args, f.CreatedTo)
	}
	if f.UpdatedSince != 0 {
		conds = append(conds, "(updated_at >= ? OR created_at >= ?)")
		args = append(args, f.UpdatedSince, f.UpdatedSince)
	}
	return strings.Join(conds, " AND "), args
}

// ListSessionsForMetrics lists sessions passing f.
func (s *SQLStore) ListSessionsForMetrics(ctx context.Context, f storage.MetricsSourceFilter) ([]*storage.AgentSession, error) {
	where, args := sourceWhere(f)
	return s.querySessions(ctx, where, args, "ORDER BY session_id")
}

// ListRunsForMetrics lists runs passing f.
func (s *SQLStore) ListRunsForMetrics(ctx context.Context, f storage.MetricsSourceFilter) ([]*storage.AgentRun, error) {
	where, args := sourceWhere(f)
	q := s.dialect.rebind(fmt.Sprintf(
		`SELECT id, session_id, user_id, run_name, run_data, metrics, created_at, updated_at FROM %s WHERE %s ORDER BY id`,
		s.runTable, where))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*storage.AgentRun
	for rows.Next() {
		var (
			r                          storage.AgentRun
			sessionID, userID, runName sql.NullString
			runData, metrics           sql.NullString
			createdAt, updatedAt       sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &sessionID, &userID, &runName, &runData, &metrics, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		r.SessionID, r.UserID, r.RunName = sessionID.String, userID.String, runName.String
		r.RunData, r.Metrics = parseMap(runData.String), parseMap(metrics.String)
		r.CreatedAt, r.UpdatedAt = time.Unix(createdAt.Int64, 0), time.Unix(updatedAt.Int64, 0)
		out = append(out, &r)
	}
	return out, rows.Err()
}

// ReplaceComponentMetrics replaces the component rows of a day in one transaction.
func (s *SQLStore) ReplaceComponentMetrics(ctx context.Context, day string, rows []storage.ComponentMetricsRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, s.dialect.rebind(fmt.Sprintf(`DELETE FROM %s WHERE metric_date = ?`, s.metricsTable())), day); err != nil {
		return err
	}
	ins := s.dialect.rebind(fmt.Sprintf(`INSERT INTO %s (id, metric_date, user_id, data, updated_at) VALUES (?, ?, ?, ?, ?)`, s.metricsTable()))
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, ins, r.ID, r.Date, r.UserID, mustJSON(r), r.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListComponentMetrics lists component rows with startDay <= date <= endDay.
func (s *SQLStore) ListComponentMetrics(ctx context.Context, startDay, endDay string) ([]storage.ComponentMetricsRow, error) {
	conds := []string{"1=1"}
	var args []interface{}
	if startDay != "" {
		conds = append(conds, "metric_date >= ?")
		args = append(args, startDay)
	}
	if endDay != "" {
		conds = append(conds, "metric_date <= ?")
		args = append(args, endDay)
	}
	q := s.dialect.rebind(fmt.Sprintf(`SELECT data FROM %s WHERE %s ORDER BY id`, s.metricsTable(), strings.Join(conds, " AND ")))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.ComponentMetricsRow
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r storage.ComponentMetricsRow
		if err := json.Unmarshal([]byte(raw), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLStore) replaceRow(ctx context.Context, table string, cols []string, vals []interface{}) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, s.dialect.rebind(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table)), vals[0]); err != nil {
		return err
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	q := s.dialect.rebind(fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(cols, ", "), ph))
	if _, err := tx.ExecContext(ctx, q, vals...); err != nil {
		return err
	}
	return tx.Commit()
}

// GetMetricsState returns the metrics calculation state.
func (s *SQLStore) GetMetricsState(ctx context.Context) (*storage.MetricsState, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, s.dialect.rebind(fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, s.stateTable())), "metrics").Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var st storage.MetricsState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// SetMetricsState stores the metrics calculation state.
func (s *SQLStore) SetMetricsState(ctx context.Context, st *storage.MetricsState) error {
	return s.replaceRow(ctx, s.stateTable(), []string{"id", "data"}, []interface{}{"metrics", mustJSON(st)})
}

// ---------------------------------------------------------------------------
// storage.KnowledgeStorage
// ---------------------------------------------------------------------------

// GetKnowledgeContent returns a knowledge row, or (nil, nil) when missing.
func (s *SQLStore) GetKnowledgeContent(id string) (*storage.KnowledgeRow, error) {
	var raw string
	err := s.db.QueryRow(s.dialect.rebind(fmt.Sprintf(`SELECT data FROM %s WHERE id = ?`, s.knowledgeTable())), id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var row storage.KnowledgeRow
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		return nil, err
	}
	return &row, nil
}

// GetKnowledgeContents returns a page of knowledge rows and the total count.
func (s *SQLStore) GetKnowledgeContents(limit, page *int, sortBy, sortOrder *string) ([]*storage.KnowledgeRow, int, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT data FROM %s`, s.knowledgeTable()))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var all []*storage.KnowledgeRow
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, 0, err
		}
		var row storage.KnowledgeRow
		if err := json.Unmarshal([]byte(raw), &row); err != nil {
			return nil, 0, err
		}
		all = append(all, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return storage.SortPaginateKnowledge(all, limit, page, sortBy, sortOrder), len(all), nil
}

// UpsertKnowledgeContent writes a knowledge row.
func (s *SQLStore) UpsertKnowledgeContent(row *storage.KnowledgeRow) (*storage.KnowledgeRow, error) {
	if row == nil || row.ID == "" {
		return nil, fmt.Errorf("sqlstore: knowledge row and id are required")
	}
	now := time.Now().Unix()
	if row.CreatedAt == nil {
		row.CreatedAt = &now
	}
	row.UpdatedAt = &now
	err := s.replaceRow(context.Background(), s.knowledgeTable(),
		[]string{"id", "name", "data", "created_at", "updated_at"},
		[]interface{}{row.ID, row.Name, mustJSON(row), *row.CreatedAt, *row.UpdatedAt})
	if err != nil {
		return nil, err
	}
	return row, nil
}

// DeleteKnowledgeContent deletes a knowledge row.
func (s *SQLStore) DeleteKnowledgeContent(id string) error {
	_, err := s.db.Exec(s.dialect.rebind(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, s.knowledgeTable())), id)
	return err
}

var (
	_ storage.SessionSearcher  = (*SQLStore)(nil)
	_ storage.MetricsStore     = (*SQLStore)(nil)
	_ storage.MetricsBackend   = (*SQLStore)(nil)
	_ storage.KnowledgeStorage = (*SQLStore)(nil)
)

package jobqueue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/internal/sqldialect"
)

// Dialect selects SQL flavour.
type Dialect = sqldialect.Dialect

// Supported dialects.
const (
	Postgres = sqldialect.Postgres
	SQLite   = sqldialect.SQLite
)

func errDuplicateID(id string) error {
	return fmt.Errorf("jobqueue: job %s already exists; ids are never reused", id)
}

func errBadStatus(s string) error { return fmt.Errorf("jobqueue: invalid settle status %q", s) }

// SQLStore is a durable Store over database/sql. On Postgres claims use
// SELECT ... FOR UPDATE SKIP LOCKED plus a per-session advisory lock; on
// SQLite claims are a transaction whose UPDATE re-checks every predicate
// (compare-and-set), which SQLite's single-writer model makes atomic. For
// SQLite set db.SetMaxOpenConns(1) or a busy_timeout to avoid SQLITE_BUSY.
type SQLStore struct {
	db      *sql.DB
	dialect Dialect
	table   string
}

// NewSQLStore creates a store over table (default "agno_jobs"); call Migrate.
func NewSQLStore(db *sql.DB, dialect Dialect, table string) (*SQLStore, error) {
	if err := dialect.Validate(); err != nil {
		return nil, err
	}
	if table == "" {
		table = "agno_jobs"
	}
	if !sqldialect.ValidIdent(table) {
		return nil, fmt.Errorf("jobqueue: invalid table name %q", table)
	}
	return &SQLStore{db: db, dialect: dialect, table: table}, nil
}

// Migrate creates the jobs table and indexes.
func (s *SQLStore) Migrate(ctx context.Context) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY, job_type TEXT NOT NULL, component_type TEXT NOT NULL, component_id TEXT NOT NULL,
			session_id TEXT NOT NULL, user_id TEXT, deployment_id TEXT, payload TEXT NOT NULL,
			serialize_session %s NOT NULL, session_seq BIGINT NOT NULL, status TEXT NOT NULL,
			attempt BIGINT NOT NULL, max_attempts BIGINT NOT NULL, idempotency_key TEXT,
			available_at BIGINT NOT NULL, locked_by TEXT, locked_at BIGINT, error TEXT,
			created_at BIGINT NOT NULL, updated_at BIGINT, completed_at BIGINT)`, s.table, s.dialect.BoolType()),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (status, available_at)`, sqldialect.IndexName(s.table, "status_available"), s.table),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (session_id)`, sqldialect.IndexName(s.table, "session"), s.table),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (idempotency_key, COALESCE(user_id, '')) WHERE idempotency_key IS NOT NULL`,
			sqldialect.IndexName(s.table, "idem"), s.table),
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("jobqueue migrate: %w", err)
		}
	}
	return nil
}

const jobCols = `id, job_type, component_type, component_id, session_id, user_id, deployment_id, payload,
	serialize_session, session_seq, status, attempt, max_attempts, idempotency_key, available_at, locked_by,
	locked_at, error, created_at, updated_at, completed_at`

type scanner interface{ Scan(...any) error }

func scanJob(r scanner) (*Job, error) {
	var (
		j                                       Job
		user, dep, payload, idem, lockedBy, err sql.NullString
		lockedAt, updated, completed            sql.NullInt64
	)
	if e := r.Scan(&j.ID, &j.JobType, &j.ComponentType, &j.ComponentID, &j.SessionID, &user, &dep, &payload,
		&j.SerializeSession, &j.SessionSeq, &j.Status, &j.Attempt, &j.MaxAttempts, &idem, &j.AvailableAt, &lockedBy,
		&lockedAt, &err, &j.CreatedAt, &updated, &completed); e != nil {
		return nil, e
	}
	j.UserID, j.DeploymentID, j.IdempotencyKey = sqldialect.StrPtr(user), sqldialect.StrPtr(dep), sqldialect.StrPtr(idem)
	j.LockedBy, j.Error = sqldialect.StrPtr(lockedBy), sqldialect.StrPtr(err)
	j.LockedAt, j.UpdatedAt, j.CompletedAt = sqldialect.IntPtr(lockedAt), sqldialect.IntPtr(updated), sqldialect.IntPtr(completed)
	if e := sqldialect.UnJSON(payload, &j.Payload); e != nil {
		return nil, e
	}
	return &j, nil
}

func (s *SQLStore) q(query string) string {
	return s.dialect.Rebind(strings.ReplaceAll(query, "{t}", s.table))
}

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *SQLStore) getWith(ctx context.Context, qr querier, id string) (*Job, error) {
	j, err := scanJob(qr.QueryRowContext(ctx, s.q(`SELECT `+jobCols+` FROM {t} WHERE id = ?`), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

// sessionLock serialises enqueue/claim for one session on Postgres.
func (s *SQLStore) sessionLock(ctx context.Context, tx *sql.Tx, sessionID string, try bool) (bool, error) {
	if s.dialect != Postgres {
		return true, nil
	}
	key := s.table + ":" + sessionID
	if try {
		var ok bool
		err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0))`, key).Scan(&ok)
		return ok, err
	}
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key)
	return err == nil, err
}

func (s *SQLStore) findDuplicate(ctx context.Context, qr querier, job *Job) (*Job, error) {
	q := `SELECT ` + jobCols + ` FROM {t} WHERE idempotency_key = ? AND user_id IS NULL`
	args := []any{*job.IdempotencyKey}
	if job.UserID != nil {
		q = `SELECT ` + jobCols + ` FROM {t} WHERE idempotency_key = ? AND user_id = ?`
		args = append(args, *job.UserID)
	}
	j, err := scanJob(qr.QueryRowContext(ctx, s.q(q), args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return j, err
}

func (s *SQLStore) Enqueue(ctx context.Context, job *Job, maxDepth int) (*EnqueueResult, error) {
	job = job.Clone()
	if job.IdempotencyKey != nil && *job.IdempotencyKey == "" {
		job.IdempotencyKey = nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	if job.IdempotencyKey != nil {
		if dup, err := s.findDuplicate(ctx, tx, job); err != nil || dup != nil {
			if err != nil {
				return nil, err
			}
			return &EnqueueResult{Reason: ReasonDuplicate, Job: dup}, nil
		}
	}
	if maxDepth > 0 {
		var n int
		if err := tx.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM {t} WHERE status = 'queued'`)).Scan(&n); err != nil {
			return nil, err
		}
		if n >= maxDepth {
			return &EnqueueResult{Reason: ReasonQueueFull}, nil
		}
	}
	if job.SerializeSession {
		if _, err := s.sessionLock(ctx, tx, job.SessionID, false); err != nil {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, s.q(`SELECT COALESCE(MAX(session_seq), 0) + 1 FROM {t}
			WHERE session_id = ? AND serialize_session = ? AND status IN ('queued','running','paused')`),
			job.SessionID, true).Scan(&job.SessionSeq); err != nil {
			return nil, err
		}
	}
	payload, err := sqldialect.JSON(job.Payload)
	if err != nil {
		return nil, err
	}
	if !payload.Valid {
		payload = sql.NullString{String: "{}", Valid: true}
	}
	_, err = tx.ExecContext(ctx, s.q(`INSERT INTO {t} (`+jobCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`),
		job.ID, job.JobType, job.ComponentType, job.ComponentID, job.SessionID, sqldialect.NullStr(job.UserID),
		sqldialect.NullStr(job.DeploymentID), payload, job.SerializeSession, job.SessionSeq, job.Status, job.Attempt,
		job.MaxAttempts, sqldialect.NullStr(job.IdempotencyKey), job.AvailableAt, sqldialect.NullStr(job.LockedBy),
		sqldialect.NullInt(job.LockedAt), sqldialect.NullStr(job.Error), job.CreatedAt, sqldialect.NullInt(job.UpdatedAt),
		sqldialect.NullInt(job.CompletedAt))
	if err != nil {
		if sqldialect.IsUniqueViolation(err) {
			_ = tx.Rollback()
			if job.IdempotencyKey != nil {
				if dup, derr := s.findDuplicate(ctx, s.db, job); derr == nil && dup != nil {
					return &EnqueueResult{Reason: ReasonDuplicate, Job: dup}, nil
				}
			}
			return nil, errDuplicateID(job.ID)
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &EnqueueResult{Accepted: true, Job: job}, nil
}

// claimable is the claim predicate over alias t; args: now, deploymentID, stale.
const claimable = `t.available_at <= ?
	AND (t.deployment_id IS NULL OR t.deployment_id = ?)
	AND (t.status = 'queued' OR (t.status = 'running' AND t.locked_at <= ? AND t.attempt < t.max_attempts))
	AND (t.serialize_session = ? OR NOT EXISTS (
		SELECT 1 FROM {t} o WHERE o.id <> t.id AND o.session_id = t.session_id AND o.serialize_session = ?
		AND (o.status IN ('running','paused') OR (t.status = 'queued' AND o.status = 'queued'
			AND (o.session_seq < t.session_seq OR (o.session_seq = t.session_seq AND o.id < t.id))))))`

func (s *SQLStore) Claim(ctx context.Context, workerID string, lockGrace time.Duration, deploymentID *string) (*Job, error) {
	now := time.Now().Unix()
	stale := now - int64(lockGrace/time.Second)
	dep := sqldialect.NullStr(deploymentID)
	predArgs := []any{now, dep, stale, false, true}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	sel := `SELECT t.id, t.session_id, t.serialize_session FROM {t} t WHERE ` + claimable + ` ORDER BY t.created_at ASC, t.id ASC LIMIT 1`
	if s.dialect == Postgres {
		sel += ` FOR UPDATE OF t SKIP LOCKED`
	}
	var (
		id, session string
		serialized  bool
	)
	if err := tx.QueryRowContext(ctx, s.q(sel), predArgs...).Scan(&id, &session, &serialized); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if serialized {
		// A concurrent claim in this session may have committed after the scan.
		if ok, err := s.sessionLock(ctx, tx, session, true); err != nil || !ok {
			return nil, err
		}
	}
	// The UPDATE re-checks every predicate: a compare-and-set on SQLite and a
	// post-lock recheck on Postgres.
	res, err := tx.ExecContext(ctx, s.q(`UPDATE {t} SET status = 'running', locked_by = ?, locked_at = ?, attempt = attempt + 1, updated_at = ?
		WHERE id = ? AND id IN (SELECT t.id FROM {t} t WHERE `+claimable+`)`),
		append([]any{workerID, now, now, id}, predArgs...)...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}
	j, err := s.getWith(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return j, tx.Commit()
}

func (s *SQLStore) Heartbeat(ctx context.Context, workerID string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := []any{time.Now().Unix(), workerID}
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := s.db.ExecContext(ctx, s.q(`UPDATE {t} SET locked_at = ? WHERE locked_by = ? AND status = 'running' AND id IN (`+ph+`)`), args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (s *SQLStore) exec(ctx context.Context, q string, args ...any) (bool, error) {
	res, err := s.db.ExecContext(ctx, s.q(q), args...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *SQLStore) Complete(ctx context.Context, id, workerID string, attempt int, status string, errMsg *string) (bool, error) {
	if !validSettle(status, true) {
		return false, errBadStatus(status)
	}
	now := time.Now().Unix()
	var completed sql.NullInt64
	if status != StatusPaused {
		completed = sql.NullInt64{Int64: now, Valid: true}
	}
	return s.exec(ctx, `UPDATE {t} SET status = ?, error = ?, locked_by = NULL, locked_at = NULL, completed_at = ?, updated_at = ?
		WHERE id = ? AND locked_by = ? AND attempt = ? AND status = 'running'`,
		status, sqldialect.NullStr(errMsg), completed, now, id, workerID, attempt)
}

func (s *SQLStore) RetryOrFail(ctx context.Context, id, workerID string, attempt int, errMsg string, delay time.Duration) (string, error) {
	now := time.Now().Unix()
	ok, err := s.exec(ctx, `UPDATE {t} SET status = 'queued', error = ?, locked_by = NULL, locked_at = NULL, available_at = ?, updated_at = ?
		WHERE id = ? AND locked_by = ? AND attempt = ? AND status = 'running' AND attempt < max_attempts`,
		errMsg, now+int64(delay/time.Second), now, id, workerID, attempt)
	if err != nil || ok {
		if ok {
			return StatusQueued, nil
		}
		return "", err
	}
	ok, err = s.exec(ctx, `UPDATE {t} SET status = 'failed', error = ?, locked_by = NULL, locked_at = NULL, completed_at = ?, updated_at = ?
		WHERE id = ? AND locked_by = ? AND attempt = ? AND status = 'running'`, errMsg, now, now, id, workerID, attempt)
	if err != nil || !ok {
		return "", err
	}
	return StatusFailed, nil
}

func (s *SQLStore) Cancel(ctx context.Context, id string) (bool, error) {
	now := time.Now().Unix()
	return s.exec(ctx, `UPDATE {t} SET status = 'cancelled', completed_at = ?, updated_at = ? WHERE id = ? AND status IN ('queued','paused')`, now, now, id)
}

func (s *SQLStore) Continue(ctx context.Context, id string, cont map[string]any) (string, *Job, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	j, err := s.getWith(ctx, tx, id)
	if errors.Is(err, ErrNotFound) {
		return ContinueConflict, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if isTerminal(j.Status) {
		return ContinueConflict, j, nil
	}
	if j.Status != StatusPaused {
		return ContinueAttach, j, nil
	}
	if j.Payload == nil {
		j.Payload = map[string]any{}
	}
	j.Payload["continue"] = cont
	payload, err := sqldialect.JSON(j.Payload)
	if err != nil {
		return "", nil, err
	}
	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, s.q(`UPDATE {t} SET status = 'queued', payload = ?, max_attempts = attempt + 1, available_at = ?,
		locked_by = NULL, locked_at = NULL, completed_at = NULL, updated_at = ? WHERE id = ? AND status = 'paused'`), payload, now, now, id)
	if err != nil {
		return "", nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ContinueAttach, j, nil
	}
	j, err = s.getWith(ctx, tx, id)
	if err != nil {
		return "", nil, err
	}
	return ContinueQueued, j, tx.Commit()
}

func (s *SQLStore) list(ctx context.Context, q string, args ...any) ([]*Job, error) {
	rows, err := s.db.QueryContext(ctx, s.q(q), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *SQLStore) SweepExhausted(ctx context.Context, lockGrace time.Duration, limit int) ([]*Job, error) {
	if limit <= 0 {
		limit = 20
	}
	stale := time.Now().Unix() - int64(lockGrace/time.Second)
	return s.list(ctx, fmt.Sprintf(`SELECT `+jobCols+` FROM {t} WHERE status = 'running' AND locked_at IS NOT NULL AND locked_at <= ?
		AND attempt >= max_attempts ORDER BY locked_at ASC LIMIT %d`, limit), stale)
}

func (s *SQLStore) AcquireSweep(ctx context.Context, id, workerID string, lockGrace time.Duration) (bool, error) {
	now := time.Now().Unix()
	return s.exec(ctx, `UPDATE {t} SET locked_by = ?, locked_at = ?, updated_at = ? WHERE id = ? AND status = 'running'
		AND locked_at IS NOT NULL AND locked_at <= ? AND attempt >= max_attempts`, workerID, now, now, id, now-int64(lockGrace/time.Second))
}

func (s *SQLStore) SettleSwept(ctx context.Context, id, workerID, status string, errMsg *string) (bool, error) {
	if !validSettle(status, true) {
		return false, errBadStatus(status)
	}
	now := time.Now().Unix()
	var completed sql.NullInt64
	if status != StatusPaused {
		completed = sql.NullInt64{Int64: now, Valid: true}
	}
	return s.exec(ctx, `UPDATE {t} SET status = ?, error = ?, locked_by = NULL, locked_at = NULL, completed_at = ?, updated_at = ?
		WHERE id = ? AND status = 'running' AND locked_by = ?`, status, sqldialect.NullStr(errMsg), completed, now, id, workerID)
}

func (s *SQLStore) Get(ctx context.Context, id string) (*Job, error) { return s.getWith(ctx, s.db, id) }

func (s *SQLStore) SessionBlocker(ctx context.Context, id string) (string, error) {
	j, err := s.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	if err != nil || j.Status != StatusQueued || !j.SerializeSession {
		return "", err
	}
	others, err := s.list(ctx, `SELECT `+jobCols+` FROM {t} WHERE session_id = ? AND serialize_session = ? AND status IN ('queued','running','paused')`,
		j.SessionID, true)
	if err != nil {
		return "", err
	}
	m := map[string]*Job{}
	for _, o := range others {
		m[o.ID] = o
	}
	return sessionBlocker(j, m), nil
}

func (s *SQLStore) CountQueued(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM {t} WHERE status = 'queued'`)).Scan(&n)
	return n, err
}

func (s *SQLStore) List(ctx context.Context, f ListFilter) ([]*Job, int, error) {
	var conds []string
	var args []any
	inList := func(col string, vals []string) {
		conds = append(conds, col+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(vals)), ",")+")")
		for _, v := range vals {
			args = append(args, v)
		}
	}
	if len(f.Statuses) > 0 {
		inList("status", f.Statuses)
	}
	if len(f.SessionIDs) > 0 {
		inList("session_id", f.SessionIDs)
	}
	if f.UserID != nil {
		conds, args = append(conds, "user_id = ?"), append(args, *f.UserID)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM {t}`+where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "created_at"
	if f.SortBy == "updated_at" {
		order = "COALESCE(updated_at, created_at)"
	}
	dir := "DESC"
	if f.SortAsc {
		dir = "ASC"
	}
	limit, page := normPage(f.Limit, f.Page)
	jobs, err := s.list(ctx, fmt.Sprintf(`SELECT `+jobCols+` FROM {t}%s ORDER BY %s %s, id ASC LIMIT %d OFFSET %d`,
		where, order, dir, limit, (page-1)*limit), args...)
	return jobs, total, err
}

func (s *SQLStore) Requeue(ctx context.Context, id string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck
	j, err := s.getWith(ctx, tx, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if j.Status != StatusFailed && j.Status != StatusCancelled {
		return false, nil
	}
	seq := j.SessionSeq
	if j.SerializeSession {
		if _, err := s.sessionLock(ctx, tx, j.SessionID, false); err != nil {
			return false, err
		}
		if err := tx.QueryRowContext(ctx, s.q(`SELECT COALESCE(MAX(session_seq), 0) + 1 FROM {t}
			WHERE session_id = ? AND serialize_session = ? AND status IN ('queued','running','paused')`), j.SessionID, true).Scan(&seq); err != nil {
			return false, err
		}
	}
	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, s.q(`UPDATE {t} SET status = 'queued', max_attempts = attempt + 1, available_at = ?, session_seq = ?,
		locked_by = NULL, locked_at = NULL, completed_at = NULL, updated_at = ? WHERE id = ? AND status IN ('failed','cancelled')`), now, seq, now, id)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return false, nil
	}
	return true, tx.Commit()
}

func (s *SQLStore) Stats(ctx context.Context) (*Stats, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT status, COUNT(*) FROM {t} GROUP BY status`))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	st := &Stats{Counts: map[string]int{}}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		st.Counts[status] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var oldest sql.NullInt64
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT MIN(created_at) FROM {t} WHERE status = 'queued'`)).Scan(&oldest); err != nil {
		return nil, err
	}
	if oldest.Valid {
		st.OldestQueuedAgeSeconds = ptr(time.Now().Unix() - oldest.Int64)
	}
	return st, nil
}

func (s *SQLStore) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM {t} WHERE status IN ('completed','failed','cancelled')
		AND completed_at IS NOT NULL AND completed_at <= ?`), time.Now().Unix()-int64(olderThan/time.Second))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

var _ Store = (*SQLStore)(nil)

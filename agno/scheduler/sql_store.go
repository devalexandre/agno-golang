package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/internal/sqldialect"
)

// Dialect re-exports the SQL dialect selector.
type Dialect = sqldialect.Dialect

// Supported SQL dialects.
const (
	Postgres = sqldialect.Postgres
	SQLite   = sqldialect.SQLite
)

// SQLStore persists schedules in Postgres or SQLite through database/sql.
// The caller supplies the *sql.DB (lib/pq / pgx stdlib for Postgres,
// modernc.org/sqlite for SQLite).
type SQLStore struct {
	db         *sql.DB
	dialect    Dialect
	schedTable string
	runsTable  string
}

// SQLStoreOptions configures table names.
type SQLStoreOptions struct {
	SchedulesTable string // default "agno_schedules"
	RunsTable      string // default "agno_schedule_runs"
}

// NewSQLStore creates a store; call Migrate once to create the tables.
func NewSQLStore(db *sql.DB, dialect Dialect, opts *SQLStoreOptions) (*SQLStore, error) {
	if err := dialect.Validate(); err != nil {
		return nil, err
	}
	s := &SQLStore{db: db, dialect: dialect, schedTable: "agno_schedules", runsTable: "agno_schedule_runs"}
	if opts != nil {
		if opts.SchedulesTable != "" {
			s.schedTable = opts.SchedulesTable
		}
		if opts.RunsTable != "" {
			s.runsTable = opts.RunsTable
		}
	}
	if !sqldialect.ValidIdent(s.schedTable) || !sqldialect.ValidIdent(s.runsTable) {
		return nil, fmt.Errorf("scheduler: invalid table name")
	}
	return s, nil
}

// Migrate creates the tables and indexes if they do not exist.
func (s *SQLStore) Migrate(ctx context.Context) error {
	b := s.dialect.BoolType()
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY, user_id TEXT, name TEXT NOT NULL, description TEXT,
			method TEXT NOT NULL, endpoint TEXT NOT NULL, payload TEXT,
			cron_expr TEXT NOT NULL DEFAULT '', interval_seconds BIGINT NOT NULL DEFAULT 0, run_at BIGINT,
			timezone TEXT NOT NULL, timeout_seconds BIGINT NOT NULL, max_retries BIGINT NOT NULL,
			retry_delay_seconds BIGINT NOT NULL, enabled %s NOT NULL, next_run_at BIGINT,
			locked_by TEXT, locked_at BIGINT, target_type TEXT, target_id TEXT, disabled_reason TEXT,
			created_at BIGINT NOT NULL, updated_at BIGINT)`, s.schedTable, b),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (name, COALESCE(user_id, ''))`,
			sqldialect.IndexName(s.schedTable, "name_owner"), s.schedTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (enabled, next_run_at)`,
			sqldialect.IndexName(s.schedTable, "due"), s.schedTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY, schedule_id TEXT NOT NULL, user_id TEXT, attempt BIGINT NOT NULL,
			triggered_at BIGINT, completed_at BIGINT, status TEXT NOT NULL, status_code BIGINT,
			run_id TEXT, session_id TEXT, error TEXT, input TEXT, output TEXT, requirements TEXT,
			created_at BIGINT NOT NULL)`, s.runsTable),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (schedule_id)`,
			sqldialect.IndexName(s.runsTable, "schedule"), s.runsTable),
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("scheduler migrate: %w", err)
		}
	}
	return nil
}

const schedCols = `id, user_id, name, description, method, endpoint, payload, cron_expr, interval_seconds, run_at,
	timezone, timeout_seconds, max_retries, retry_delay_seconds, enabled, next_run_at, locked_by, locked_at,
	target_type, target_id, disabled_reason, created_at, updated_at`

const runCols = `id, schedule_id, user_id, attempt, triggered_at, completed_at, status, status_code, run_id,
	session_id, error, input, output, requirements, created_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanSchedule(r rowScanner) (*Schedule, error) {
	var (
		s                                                   Schedule
		userID, desc, payload, lockedBy, tt, tid, disReason sql.NullString
		runAt, next, lockedAt, updated                      sql.NullInt64
	)
	err := r.Scan(&s.ID, &userID, &s.Name, &desc, &s.Method, &s.Endpoint, &payload, &s.CronExpr, &s.IntervalSeconds,
		&runAt, &s.Timezone, &s.TimeoutSeconds, &s.MaxRetries, &s.RetryDelaySeconds, &s.Enabled, &next, &lockedBy,
		&lockedAt, &tt, &tid, &disReason, &s.CreatedAt, &updated)
	if err != nil {
		return nil, err
	}
	s.UserID, s.Description, s.LockedBy = sqldialect.StrPtr(userID), sqldialect.StrPtr(desc), sqldialect.StrPtr(lockedBy)
	s.TargetType, s.TargetID, s.DisabledReason = sqldialect.StrPtr(tt), sqldialect.StrPtr(tid), sqldialect.StrPtr(disReason)
	s.RunAt, s.NextRunAt, s.LockedAt, s.UpdatedAt = sqldialect.IntPtr(runAt), sqldialect.IntPtr(next), sqldialect.IntPtr(lockedAt), sqldialect.IntPtr(updated)
	if err := sqldialect.UnJSON(payload, &s.Payload); err != nil {
		return nil, err
	}
	return &s, nil
}

func scanRun(r rowScanner) (*ScheduleRun, error) {
	var (
		run                                        ScheduleRun
		userID, runID, sessID, errS, in, out, reqs sql.NullString
		trig, comp, code                           sql.NullInt64
	)
	err := r.Scan(&run.ID, &run.ScheduleID, &userID, &run.Attempt, &trig, &comp, &run.Status, &code, &runID, &sessID,
		&errS, &in, &out, &reqs, &run.CreatedAt)
	if err != nil {
		return nil, err
	}
	run.UserID, run.RunID, run.SessionID, run.Error = sqldialect.StrPtr(userID), sqldialect.StrPtr(runID), sqldialect.StrPtr(sessID), sqldialect.StrPtr(errS)
	run.TriggeredAt, run.CompletedAt = sqldialect.IntPtr(trig), sqldialect.IntPtr(comp)
	if code.Valid {
		c := int(code.Int64)
		run.StatusCode = &c
	}
	for _, p := range []struct {
		s   sql.NullString
		dst any
	}{{in, &run.Input}, {out, &run.Output}, {reqs, &run.Requirements}} {
		if err := sqldialect.UnJSON(p.s, p.dst); err != nil {
			return nil, err
		}
	}
	return &run, nil
}

func (s *SQLStore) q(query string) string { return s.dialect.Rebind(query) }

func (s *SQLStore) CreateSchedule(ctx context.Context, sc *Schedule) error {
	payload, err := sqldialect.JSON(sc.Payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, s.q(fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.schedTable, schedCols)),
		sc.ID, sqldialect.NullStr(sc.UserID), sc.Name, sqldialect.NullStr(sc.Description), sc.Method, sc.Endpoint, payload,
		sc.CronExpr, sc.IntervalSeconds, sqldialect.NullInt(sc.RunAt), sc.Timezone, sc.TimeoutSeconds, sc.MaxRetries,
		sc.RetryDelaySeconds, sc.Enabled, sqldialect.NullInt(sc.NextRunAt), sqldialect.NullStr(sc.LockedBy),
		sqldialect.NullInt(sc.LockedAt), sqldialect.NullStr(sc.TargetType), sqldialect.NullStr(sc.TargetID),
		sqldialect.NullStr(sc.DisabledReason), sc.CreatedAt, sqldialect.NullInt(sc.UpdatedAt))
	if sqldialect.IsUniqueViolation(err) {
		return ErrDuplicateName
	}
	return err
}

func (s *SQLStore) getOne(ctx context.Context, where string, args ...any) (*Schedule, error) {
	row := s.db.QueryRowContext(ctx, s.q(fmt.Sprintf(`SELECT %s FROM %s WHERE %s`, schedCols, s.schedTable, where)), args...)
	sc, err := scanSchedule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sc, err
}

func (s *SQLStore) GetSchedule(ctx context.Context, id string, userID *string) (*Schedule, error) {
	if userID != nil {
		return s.getOne(ctx, "id = ? AND user_id = ?", id, *userID)
	}
	return s.getOne(ctx, "id = ?", id)
}

func (s *SQLStore) GetScheduleByName(ctx context.Context, name string, userID *string) (*Schedule, error) {
	if userID != nil {
		return s.getOne(ctx, "name = ? AND user_id = ?", name, *userID)
	}
	return s.getOne(ctx, "name = ? AND user_id IS NULL", name)
}

func (s *SQLStore) ListSchedules(ctx context.Context, f ListFilter) ([]*Schedule, int, error) {
	var conds []string
	var args []any
	if f.UserID != nil {
		conds, args = append(conds, "user_id = ?"), append(args, *f.UserID)
	}
	if f.Enabled != nil {
		conds, args = append(conds, "enabled = ?"), append(args, *f.Enabled)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, s.q(fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, s.schedTable, where)), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, page := f.Limit, f.Page
	if limit <= 0 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	rows, err := s.db.QueryContext(ctx, s.q(fmt.Sprintf(`SELECT %s FROM %s%s ORDER BY created_at DESC, id ASC LIMIT %d OFFSET %d`,
		schedCols, s.schedTable, where, limit, (page-1)*limit)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, sc)
	}
	return out, total, rows.Err()
}

func (s *SQLStore) UpdateSchedule(ctx context.Context, sc *Schedule) error {
	payload, err := sqldialect.JSON(sc.Payload)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, s.q(fmt.Sprintf(`UPDATE %s SET name=?, description=?, method=?, endpoint=?, payload=?,
		cron_expr=?, interval_seconds=?, run_at=?, timezone=?, timeout_seconds=?, max_retries=?, retry_delay_seconds=?,
		enabled=?, next_run_at=?, target_type=?, target_id=?, disabled_reason=?, updated_at=? WHERE id=?`, s.schedTable)),
		sc.Name, sqldialect.NullStr(sc.Description), sc.Method, sc.Endpoint, payload, sc.CronExpr, sc.IntervalSeconds,
		sqldialect.NullInt(sc.RunAt), sc.Timezone, sc.TimeoutSeconds, sc.MaxRetries, sc.RetryDelaySeconds, sc.Enabled,
		sqldialect.NullInt(sc.NextRunAt), sqldialect.NullStr(sc.TargetType), sqldialect.NullStr(sc.TargetID),
		sqldialect.NullStr(sc.DisabledReason), sqldialect.NullInt(sc.UpdatedAt), sc.ID)
	if sqldialect.IsUniqueViolation(err) {
		return ErrDuplicateName
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) DeleteSchedule(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, s.q(fmt.Sprintf(`DELETE FROM %s WHERE schedule_id = ?`, s.runsTable)), id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, s.q(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, s.schedTable)), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// ClaimDueSchedule uses FOR UPDATE SKIP LOCKED on Postgres and a
// compare-and-set update inside a transaction on SQLite.
func (s *SQLStore) ClaimDueSchedule(ctx context.Context, workerID string, lockGrace time.Duration) (*Schedule, error) {
	now := time.Now().Unix()
	stale := now - int64(lockGrace/time.Second)
	due := `enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ? AND (locked_by IS NULL OR locked_at <= ?)`
	if s.dialect == Postgres {
		q := fmt.Sprintf(`UPDATE %[1]s SET locked_by = ?, locked_at = ? WHERE id = (
			SELECT id FROM %[1]s WHERE %[2]s ORDER BY next_run_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED)
			RETURNING %[3]s`, s.schedTable, due, schedCols)
		sc, err := scanSchedule(s.db.QueryRowContext(ctx, s.q(q), workerID, now, true, now, stale))
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return sc, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	var id string
	err = tx.QueryRowContext(ctx, s.q(fmt.Sprintf(`SELECT id FROM %s WHERE %s ORDER BY next_run_at ASC LIMIT 1`, s.schedTable, due)),
		true, now, stale).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, s.q(fmt.Sprintf(`UPDATE %s SET locked_by = ?, locked_at = ? WHERE id = ? AND %s`, s.schedTable, due)),
		workerID, now, id, true, now, stale)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}
	sc, err := scanSchedule(tx.QueryRowContext(ctx, s.q(fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, schedCols, s.schedTable)), id))
	if err != nil {
		return nil, err
	}
	return sc, tx.Commit()
}

func (s *SQLStore) ReleaseSchedule(ctx context.Context, id string, r Release) error {
	set := "locked_by = NULL, locked_at = NULL, next_run_at = ?, updated_at = ?"
	args := []any{sqldialect.NullInt(r.NextRunAt), time.Now().Unix()}
	if r.Disable {
		set += ", enabled = ?, disabled_reason = ?"
		args = append(args, false, sqldialect.NullStr(r.DisabledReason))
	}
	where := "id = ?"
	args = append(args, id)
	if r.WorkerID != "" {
		where += " AND locked_by = ?"
		args = append(args, r.WorkerID)
	}
	if r.LockedAt != nil {
		where += " AND locked_at = ?"
		args = append(args, *r.LockedAt)
	}
	_, err := s.db.ExecContext(ctx, s.q(fmt.Sprintf(`UPDATE %s SET %s WHERE %s`, s.schedTable, set, where)), args...)
	return err
}

func runArgs(r *ScheduleRun) ([]any, error) {
	in, err := sqldialect.JSON(r.Input)
	if err != nil {
		return nil, err
	}
	out, err := sqldialect.JSON(r.Output)
	if err != nil {
		return nil, err
	}
	var reqs sql.NullString
	if len(r.Requirements) > 0 {
		if reqs, err = sqldialect.JSON(r.Requirements); err != nil {
			return nil, err
		}
	}
	var code sql.NullInt64
	if r.StatusCode != nil {
		code = sql.NullInt64{Int64: int64(*r.StatusCode), Valid: true}
	}
	return []any{sqldialect.NullStr(r.UserID), r.Attempt, sqldialect.NullInt(r.TriggeredAt), sqldialect.NullInt(r.CompletedAt),
		r.Status, code, sqldialect.NullStr(r.RunID), sqldialect.NullStr(r.SessionID), sqldialect.NullStr(r.Error), in, out, reqs}, nil
}

func (s *SQLStore) CreateRun(ctx context.Context, r *ScheduleRun) error {
	args, err := runArgs(r)
	if err != nil {
		return err
	}
	args = append([]any{r.ID, r.ScheduleID}, args...)
	args = append(args, r.CreatedAt)
	_, err = s.db.ExecContext(ctx, s.q(fmt.Sprintf(`INSERT INTO %s (%s) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, s.runsTable, runCols)), args...)
	return err
}

func (s *SQLStore) UpdateRun(ctx context.Context, r *ScheduleRun) error {
	args, err := runArgs(r)
	if err != nil {
		return err
	}
	args = append(args, r.ID)
	res, err := s.db.ExecContext(ctx, s.q(fmt.Sprintf(`UPDATE %s SET user_id=?, attempt=?, triggered_at=?, completed_at=?, status=?,
		status_code=?, run_id=?, session_id=?, error=?, input=?, output=?, requirements=? WHERE id=?`, s.runsTable)), args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) GetRun(ctx context.Context, id string, userID *string) (*ScheduleRun, error) {
	q := fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, runCols, s.runsTable)
	args := []any{id}
	if userID != nil {
		q += " AND user_id = ?"
		args = append(args, *userID)
	}
	r, err := scanRun(s.db.QueryRowContext(ctx, s.q(q), args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

func (s *SQLStore) ListRuns(ctx context.Context, scheduleID string, userID *string, limit, page int) ([]*ScheduleRun, int, error) {
	where := " WHERE schedule_id = ?"
	args := []any{scheduleID}
	if userID != nil {
		where += " AND user_id = ?"
		args = append(args, *userID)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, s.q(fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, s.runsTable, where)), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	rows, err := s.db.QueryContext(ctx, s.q(fmt.Sprintf(`SELECT %s FROM %s%s ORDER BY id DESC LIMIT %d OFFSET %d`,
		runCols, s.runsTable, where, limit, (page-1)*limit)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*ScheduleRun
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

var _ Store = (*SQLStore)(nil)
var _ Store = (*MemoryStore)(nil)

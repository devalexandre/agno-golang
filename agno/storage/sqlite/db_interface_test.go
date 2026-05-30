package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// TestSessionRoundTripWithDefaultDriver guards two regressions:
//  1. The default database/sql driver must be the pure-Go "sqlite" driver
//     (modernc.org/sqlite), not the previously hardcoded "sqlite3". Opening a
//     DBFile without a CGO driver used to fail with `unknown driver "sqlite3"`.
//  2. ReadSession must select explicit columns in Scan order. With SELECT *,
//     the AgentMode schema returns agent_data/agent_id after the common
//     columns, so positional scanning misaligned and failed converting the
//     agent_data JSON ("{}") into the created_at int64.
func TestSessionRoundTripWithDefaultDriver(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "sessions.db")
	db, err := NewSqliteStorage(SqliteStorageConfig{
		TableName: "agno_sessions",
		DBFile:    &dbFile, // no injected *sql.DB and no Driver -> exercises the default driver
	})
	if err != nil {
		t.Fatalf("NewSqliteStorage: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.CreateTables(ctx); err != nil {
		t.Fatalf("CreateTables: %v", err)
	}

	sess := storage.NewAgentSession("sess-1", "user-1", "agent-1")
	sess.AgentData = map[string]interface{}{"role": "explorer"}
	sess.SessionData = map[string]interface{}{"k": "v"}
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := db.ReadSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if got.SessionID != "sess-1" || got.UserID != "user-1" || got.AgentID != "agent-1" {
		t.Fatalf("session fields misaligned: %+v", got)
	}
	if got.AgentData["role"] != "explorer" {
		t.Fatalf("agent_data not round-tripped: %+v", got.AgentData)
	}
	if got.SessionData["k"] != "v" {
		t.Fatalf("session_data not round-tripped: %+v", got.SessionData)
	}

	// Not-found path must return the sentinel message loadSession relies on.
	if _, err := db.ReadSession(ctx, "missing"); err == nil || err.Error() != "session not found: missing" {
		t.Fatalf("expected 'session not found' error, got: %v", err)
	}
}

// TestRunRoundTrip verifies CreateRun + GetRunsForSession, the path the agent
// uses to rebuild conversation history from stored runs.
func TestRunRoundTrip(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "runs.db")
	db, err := NewSqliteStorage(SqliteStorageConfig{
		TableName: "agno_sessions",
		DBFile:    &dbFile,
	})
	if err != nil {
		t.Fatalf("NewSqliteStorage: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.CreateTables(ctx); err != nil {
		t.Fatalf("CreateTables: %v", err)
	}

	run := &storage.AgentRun{
		SessionID:    "sess-1",
		UserID:       "user-1",
		UserMessage:  "hello",
		AgentMessage: "hi there",
	}
	if err := db.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	runs, err := db.GetRunsForSession(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetRunsForSession: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].UserMessage != "hello" || runs[0].AgentMessage != "hi there" {
		t.Fatalf("run fields misaligned: %+v", runs[0])
	}
}

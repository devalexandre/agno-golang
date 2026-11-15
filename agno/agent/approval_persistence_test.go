package agent

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// A required approval created by one process is resolved and continued by
// another: the agent DB (sqlite file) stores both the paused run and the
// approval record.
func TestRequiredApprovalPersistsInAgentDB(t *testing.T) {
	ctx := context.Background()
	dbFile := filepath.Join(t.TempDir(), "agent.db")
	open := func() *sqlite.SqliteStorage {
		db, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{TableName: "agno_sessions", DBFile: &dbFile})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.CreateTables(ctx); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}

	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithApproval(toolkit.ApprovalRequired)}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"to":"bob","subject":"s"}`)}}

	// Process 1: the run pauses and a pending approval is written to the DB.
	db1 := open()
	a1 := newHITLAgent(t, m, tool, func(c *AgentConfig) { c.DB = db1; c.SessionID = "sess-1" })
	resp, err := a1.Run("email bob")
	if err != nil || !resp.IsPaused() {
		t.Fatalf("expected pause: %v %s", err, resp.Status)
	}
	approvalID := resp.Requirements[0].ToolExecution.ApprovalID
	pending, err := db1.ListApprovals(ctx, approval.Filter{Status: approval.StatusPending})
	if err != nil || len(pending) != 1 || pending[0].ID != approvalID || pending[0].RunID != resp.RunID {
		t.Fatalf("pending approval should be in the DB: %v %v", pending, err)
	}

	// Process 2: an admin approves through a fresh connection, then a fresh
	// agent continues the run.
	db2 := open()
	a2 := newHITLAgent(t, m, tool, func(c *AgentConfig) { c.DB = db2; c.SessionID = "sess-1" })
	if _, err := a2.ContinueRun(ctx, resp.RunID); !errors.Is(err, ErrApprovalPending) {
		t.Fatalf("expected ErrApprovalPending before resolution, got %v", err)
	}
	if _, err := approval.Resolve(ctx, db2, approvalID, approval.StatusApproved, "admin", nil); err != nil {
		t.Fatal(err)
	}
	out, err := a2.ContinueRun(ctx, resp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != models.RunStatusCompleted || len(tool.called()) != 1 {
		t.Fatalf("expected the approved call to run once: %s %v", out.Status, tool.called())
	}
	rec, err := db2.GetApproval(ctx, approvalID)
	if err != nil || rec.Status != approval.StatusApproved || rec.RunStatus != models.RunStatusCompleted {
		t.Fatalf("approval record should be approved/completed: %+v %v", rec, err)
	}
}

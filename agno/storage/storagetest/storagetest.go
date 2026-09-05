// Package storagetest provides a reusable conformance suite for storage.DB
// implementations, so every backend (in_memory, json, postgres, mysql, …) is
// validated against the same contract.
package storagetest

import (
	"context"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// RunDBConformance exercises the full storage.DB contract against db.
func RunDBConformance(t *testing.T, db storage.DB) {
	t.Helper()
	ctx := context.Background()

	if err := db.CreateTables(ctx); err != nil {
		t.Fatalf("CreateTables: %v", err)
	}

	// --- sessions ---
	sess := storage.NewAgentSession("s1", "u1", "agent-a")
	sess.Memory["k"] = "v"
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := db.ReadSession(ctx, "s1")
	if err != nil || got == nil {
		t.Fatalf("ReadSession: got=%v err=%v", got, err)
	}
	if got.UserID != "u1" || got.AgentID != "agent-a" {
		t.Fatalf("ReadSession mismatch: %+v", got)
	}
	if got.Memory["k"] != "v" {
		t.Fatalf("ReadSession memory not persisted: %+v", got.Memory)
	}

	// missing session -> (nil, nil)
	if miss, err := db.ReadSession(ctx, "nope"); err != nil || miss != nil {
		t.Fatalf("ReadSession(missing) = %v, %v; want nil, nil", miss, err)
	}

	got.AgentData["role"] = "lead"
	if err := db.UpdateSession(ctx, got); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	updated, _ := db.ReadSession(ctx, "s1")
	if updated.AgentData["role"] != "lead" {
		t.Fatalf("UpdateSession not applied: %+v", updated.AgentData)
	}

	_ = db.CreateSession(ctx, storage.NewAgentSession("s2", "u1", "agent-a"))
	_ = db.CreateSession(ctx, storage.NewAgentSession("s3", "u2", "agent-b"))

	ids, err := db.GetAllSessionIDs(ctx, "u1")
	if err != nil {
		t.Fatalf("GetAllSessionIDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != "s1" || ids[1] != "s2" {
		t.Fatalf("GetAllSessionIDs(u1) = %v", ids)
	}

	all, err := db.GetAllSessions(ctx, "")
	if err != nil || len(all) != 3 {
		t.Fatalf("GetAllSessions(all) = %d sessions, err=%v", len(all), err)
	}

	if err := db.DeleteSession(ctx, "s3"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if gone, _ := db.ReadSession(ctx, "s3"); gone != nil {
		t.Fatalf("DeleteSession did not remove s3")
	}

	// --- runs ---
	run := &storage.AgentRun{
		ID:           "r1",
		SessionID:    "s1",
		UserID:       "u1",
		RunName:      "first",
		UserMessage:  "hi",
		AgentMessage: "hello",
		RunData:      map[string]interface{}{"x": float64(1)},
		Messages:     []map[string]interface{}{{"role": "user", "content": "hi"}},
		Metrics:      map[string]interface{}{"tokens": float64(10)},
		CreatedAt:    time.Unix(1000, 0),
		UpdatedAt:    time.Unix(1000, 0),
	}
	if err := db.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	gotRun, err := db.ReadRun(ctx, "r1")
	if err != nil || gotRun == nil {
		t.Fatalf("ReadRun: got=%v err=%v", gotRun, err)
	}
	if gotRun.UserMessage != "hi" || gotRun.AgentMessage != "hello" {
		t.Fatalf("ReadRun mismatch: %+v", gotRun)
	}
	if len(gotRun.Messages) != 1 || gotRun.Messages[0]["content"] != "hi" {
		t.Fatalf("ReadRun messages not persisted: %+v", gotRun.Messages)
	}

	gotRun.AgentMessage = "updated"
	if err := db.UpdateRun(ctx, gotRun); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	if r, _ := db.ReadRun(ctx, "r1"); r.AgentMessage != "updated" {
		t.Fatalf("UpdateRun not applied: %+v", r)
	}

	_ = db.CreateRun(ctx, &storage.AgentRun{ID: "r2", SessionID: "s1", CreatedAt: time.Unix(2000, 0), UpdatedAt: time.Unix(2000, 0)})
	_ = db.CreateRun(ctx, &storage.AgentRun{ID: "r3", SessionID: "s2", CreatedAt: time.Unix(3000, 0), UpdatedAt: time.Unix(3000, 0)})

	runs, err := db.GetRunsForSession(ctx, "s1")
	if err != nil || len(runs) != 2 {
		t.Fatalf("GetRunsForSession(s1) = %d runs, err=%v", len(runs), err)
	}

	if err := db.DeleteRun(ctx, "r1"); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
	if r, _ := db.ReadRun(ctx, "r1"); r != nil {
		t.Fatalf("DeleteRun did not remove r1")
	}
}

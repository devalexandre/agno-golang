// Package approvaltest provides a conformance suite for approval.Store
// implementations.
package approvaltest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
)

// RunStoreConformance checks the approval.Store contract on s, which must be
// empty.
func RunStoreConformance(t *testing.T, s approval.Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Unix()

	te := &models.ToolExecution{ToolCallID: "call_1", ToolName: "Ops_Send", ToolArgs: map[string]interface{}{"to": "bob"}, RequiresConfirmation: true}
	req := models.NewRunRequirement(te)

	first := &approval.Approval{
		ID: "ap_1", RunID: "run_1", SessionID: "s1", Status: approval.StatusPending,
		ApprovalType: approval.TypeRequired, PauseType: models.PauseTypeConfirmation,
		ToolName: "Ops_Send", ToolArgs: map[string]interface{}{"to": "bob"},
		SourceType: "agent", AgentID: "agent_1", UserID: "u1", SourceName: "ops",
		Requirements: []*models.RunRequirement{req},
		Context:      map[string]interface{}{"note": "first"},
		CreatedAt:    now - 10,
	}
	second := &approval.Approval{
		ID: "ap_2", RunID: "run_2", SessionID: "s1", Status: approval.StatusApproved,
		ApprovalType: approval.TypeAudit, PauseType: models.PauseTypeConfirmation,
		SourceType: "agent", CreatedAt: now,
	}

	t.Run("create and get", func(t *testing.T) {
		if err := s.CreateApproval(ctx, nil); err == nil {
			t.Fatal("nil approval must be refused")
		}
		for _, a := range []*approval.Approval{first, second} {
			if err := s.CreateApproval(ctx, a); err != nil {
				t.Fatalf("create %s: %v", a.ID, err)
			}
		}
		got, err := s.GetApproval(ctx, "ap_1")
		if err != nil {
			t.Fatal(err)
		}
		if got.RunID != "run_1" || got.Status != approval.StatusPending || got.ApprovalType != approval.TypeRequired ||
			got.ToolName != "Ops_Send" || got.ToolArgs["to"] != "bob" || got.AgentID != "agent_1" || got.UserID != "u1" ||
			got.Context["note"] != "first" || got.CreatedAt != now-10 {
			t.Fatalf("round trip mismatch: %+v", got)
		}
		if len(got.Requirements) != 1 || got.Requirements[0].ToolExecution == nil ||
			got.Requirements[0].ToolExecution.ToolCallID != "call_1" {
			t.Fatalf("requirements not preserved: %+v", got.Requirements)
		}
		if _, err := s.GetApproval(ctx, "missing"); !errors.Is(err, approval.ErrNotFound) {
			t.Fatalf("missing approval: want ErrNotFound, got %v", err)
		}
	})

	t.Run("list filters newest first", func(t *testing.T) {
		all, err := s.ListApprovals(ctx, approval.Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 2 || all[0].ID != "ap_2" || all[1].ID != "ap_1" {
			t.Fatalf("want [ap_2 ap_1], got %v", ids(all))
		}
		for name, tc := range map[string]struct {
			f    approval.Filter
			want []string
		}{
			"run":     {approval.Filter{RunID: "run_1"}, []string{"ap_1"}},
			"session": {approval.Filter{SessionID: "s1"}, []string{"ap_2", "ap_1"}},
			"status":  {approval.Filter{Status: approval.StatusPending}, []string{"ap_1"}},
			"type":    {approval.Filter{ApprovalType: approval.TypeAudit}, []string{"ap_2"}},
			"limit":   {approval.Filter{Limit: 1}, []string{"ap_2"}},
			"none":    {approval.Filter{RunID: "nope"}, nil},
		} {
			got, err := s.ListApprovals(ctx, tc.f)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !equal(ids(got), tc.want) {
				t.Fatalf("%s: want %v, got %v", name, tc.want, ids(got))
			}
		}
	})

	t.Run("resolve once", func(t *testing.T) {
		res, err := approval.Resolve(ctx, s, "ap_1", approval.StatusApproved, "alice", map[string]interface{}{"note": "ok"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Status != approval.StatusApproved || res.ResolvedBy != "alice" || res.ResolvedAt == 0 || res.ResolutionData["note"] != "ok" {
			t.Fatalf("bad resolution: %+v", res)
		}
		if _, err := approval.Resolve(ctx, s, "ap_1", approval.StatusRejected, "bob", nil); !errors.Is(err, approval.ErrStatusConflict) {
			t.Fatalf("second resolution: want ErrStatusConflict, got %v", err)
		}
		if _, err := approval.Resolve(ctx, s, "missing", approval.StatusApproved, "bob", nil); !errors.Is(err, approval.ErrNotFound) {
			t.Fatalf("missing: want ErrNotFound, got %v", err)
		}
		got, _ := s.GetApproval(ctx, "ap_1")
		if got.Status != approval.StatusApproved || got.ResolvedBy != "alice" {
			t.Fatalf("resolution not persisted: %+v", got)
		}
	})

	t.Run("concurrent resolution has one winner", func(t *testing.T) {
		if err := s.CreateApproval(ctx, &approval.Approval{
			ID: "ap_race", RunID: "run_3", SessionID: "s2", Status: approval.StatusPending,
			ApprovalType: approval.TypeRequired, PauseType: models.PauseTypeConfirmation, SourceType: "agent", CreatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := approval.Resolve(ctx, s, "ap_race", approval.StatusApproved, "x", nil); err == nil {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("want exactly one winner, got %d", wins)
		}
	})

	t.Run("run status", func(t *testing.T) {
		n, err := s.UpdateRunStatus(ctx, "run_1", models.RunStatusCompleted)
		if err != nil || n != 1 {
			t.Fatalf("want 1 updated, got %d %v", n, err)
		}
		if n, _ := s.UpdateRunStatus(ctx, "run_1", models.RunStatusCompleted); n != 0 {
			t.Fatalf("unchanged status should update 0, got %d", n)
		}
		got, _ := s.GetApproval(ctx, "ap_1")
		if got.RunStatus != models.RunStatusCompleted {
			t.Fatalf("run status not stamped: %q", got.RunStatus)
		}
	})
}

func ids(as []*approval.Approval) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.ID)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

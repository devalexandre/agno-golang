package component

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestFromRunResponseStatus(t *testing.T) {
	res := FromRunResponse(models.RunResponse{TextContent: "x", Status: models.RunStatusPaused,
		Tools: []map[string]interface{}{{"tool_call_id": "c1", "tool_name": "delete"}}})
	if res.Status != StatusPaused || len(res.Requirements) != 1 {
		t.Fatalf("paused: %+v", res)
	}
	if r := FromRunResponse(models.RunResponse{TextContent: "ok"}); r.Status != StatusCompleted {
		t.Fatalf("default: %+v", r)
	}
	if r := FromRunResponse(models.RunResponse{Status: models.RunStatusCancelled}); r.Status != StatusCancelled {
		t.Fatalf("cancelled: %+v", r)
	}
}

func TestStreamFallbackAndRegistry(t *testing.T) {
	f := &Func{IDValue: "a1", NameValue: "Alpha", RunFunc: func(_ context.Context, r RunRequest) (*RunResult, error) {
		return &RunResult{Content: "hi " + r.Input}, nil
	}}
	var types []EventType
	res, err := Stream(context.Background(), f, RunRequest{Input: "x", SessionID: "s"}, func(e Event) error {
		types = append(types, e.Type)
		return nil
	})
	if err != nil || res.Content != "hi x" || res.SessionID != "s" || res.ComponentID != "a1" || res.RunID == "" {
		t.Fatalf("result: %+v %v", res, err)
	}
	if len(types) != 3 || types[0] != EventRunStarted || types[2] != EventRunCompleted {
		t.Fatalf("events: %v", types)
	}
	reg := NewRegistry(f)
	if c, ok := reg.Get(KindAgent, "Alpha"); !ok || c.ID() != "a1" {
		t.Fatal("lookup by name")
	}
	if _, err := Continue(context.Background(), f, ContinueRequest{}); err != ErrNotSupported {
		t.Fatalf("continue: %v", err)
	}
	if DeterministicID("agent", "Assistant") != DeterministicID("agent", "Assistant") {
		t.Fatal("deterministic id")
	}
}

func TestPaginate(t *testing.T) {
	s := []SessionInfo{{SessionID: "a", CreatedAt: 1}, {SessionID: "b", CreatedAt: 3}, {SessionID: "c", CreatedAt: 2}}
	page, total, _ := Paginate(s, SessionQuery{Limit: 2, Page: 1})
	if total != 3 || len(page) != 2 || page[0].SessionID != "b" {
		t.Fatalf("page: %+v", page)
	}
	page, _, _ = Paginate(s, SessionQuery{Limit: 2, Page: 2, SortOrder: "asc"})
	if len(page) != 1 || page[0].SessionID != "b" {
		t.Fatalf("page 2 asc: %+v", page)
	}
}

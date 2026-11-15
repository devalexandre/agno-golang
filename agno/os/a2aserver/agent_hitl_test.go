package a2aserver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/internal/hitltest"
)

// A real agent pauses on a confirmation-gated tool: the task goes to
// input-required with the real RunRequirement fields in its data part, and the
// resolved requirement sent back continues the run through Agent.ContinueRun.
func TestRealAgentInputRequiredAndContinue(t *testing.T) {
	ag, files, err := hitltest.NewAgent("cleaner")
	if err != nil {
		t.Fatal(err)
	}
	comp := component.FromAgent(ag)
	srv, err := New(component.NewRegistry(comp), nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	c := a2a.NewClient(ts.URL + "/a2a/agents/" + comp.ID())
	ctx := context.Background()

	res, err := c.SendMessage(ctx, "delete /tmp/x", a2a.SendOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsInputRequired() || !strings.Contains(res.Content, hitltest.ToolName) {
		t.Fatalf("paused: %+v", res)
	}
	reqs := res.Data.(map[string]any)["requirements"].([]any)
	if len(reqs) != 1 {
		t.Fatalf("requirements: %v", reqs)
	}
	req := reqs[0].(map[string]any)
	te, _ := req["tool_execution"].(map[string]any)
	if req["id"] == "" || req["pause_type"] != "confirmation" || te["tool_name"] != hitltest.ToolName ||
		te["requires_confirmation"] != true || te["tool_args"].(map[string]any)["path"] != "/tmp/x" {
		t.Fatalf("requirement fields: %v", req)
	}
	if len(files.Deleted()) != 0 {
		t.Fatal("tool ran before confirmation")
	}

	// Unresolved requirement: Agent.ContinueRun refuses, the task keeps waiting.
	again, err := c.SendMessage(ctx, "", a2a.SendOptions{TaskID: res.TaskID, Data: map[string]any{"requirements": reqs}})
	if err != nil || !again.IsInputRequired() || !strings.Contains(again.Content, "not resolved") {
		t.Fatalf("unresolved continue: %+v %v", again, err)
	}

	req["confirmation"] = true
	done, err := c.SendMessage(ctx, "", a2a.SendOptions{TaskID: res.TaskID, Data: map[string]any{"requirements": reqs}})
	if err != nil || !done.IsCompleted() || done.Content != "final: deleted /tmp/x" {
		t.Fatalf("continue: %+v %v", done, err)
	}
	if got := files.Deleted(); len(got) != 1 || got[0] != "/tmp/x" {
		t.Fatalf("deleted: %v", got)
	}
}

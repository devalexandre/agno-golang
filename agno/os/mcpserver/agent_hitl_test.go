package mcpserver

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/internal/hitltest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// run_agent pauses a real agent on a confirmation-gated tool; continue_run
// with the requirement resolved (confirmation=true) resumes it.
func TestRealAgentContinueRun(t *testing.T) {
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
	t.Cleanup(ts.Close)
	cs := connect(t, ts.URL+"/mcp")
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "run_agent", Arguments: map[string]any{"agent_id": comp.ID(), "message": "delete /tmp/x"}})
	if err != nil {
		t.Fatal(err)
	}
	sc := res.StructuredContent.(map[string]any)
	reqs, _ := sc["requirements"].([]any)
	if sc["status"] != "PAUSED" || len(reqs) != 1 {
		t.Fatalf("paused: %v", sc)
	}
	reqs[0].(map[string]any)["confirmation"] = true
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "continue_run", Arguments: map[string]any{
		"run_id": sc["run_id"], "session_id": sc["session_id"], "agent_id": comp.ID(), "requirements": reqs}})
	if err != nil || res.IsError || text(res) != "final: deleted /tmp/x" {
		t.Fatalf("continue: %v %+v %q", err, res, text(res))
	}
	if len(files.Deleted()) != 1 {
		t.Fatalf("deleted: %v", files.Deleted())
	}
}

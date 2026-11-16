package component_test

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/internal/hitltest"
)

// Pause -> wire requirements -> resolve in wire form -> Agent.ContinueRun.
func TestAgentComponentContinueRun(t *testing.T) {
	ag, files, err := hitltest.NewAgent("cleaner")
	if err != nil {
		t.Fatal(err)
	}
	c := component.FromAgent(ag)
	ctx := context.Background()
	res, err := c.Run(ctx, component.RunRequest{Input: "delete /tmp/x", SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != component.StatusPaused || len(res.Requirements) != 1 || res.Requirements[0]["pause_type"] != "confirmation" {
		t.Fatalf("paused: %+v", res)
	}
	reqs := res.Requirements
	reqs[0]["confirmation"] = false
	reqs[0]["confirmation_note"] = "keep it"
	out, err := component.Continue(ctx, c, component.ContinueRequest{RunID: res.RunID, SessionID: res.SessionID, Requirements: reqs})
	if err != nil {
		t.Fatal(err)
	}
	// A completed run that went through a pause carries no requirements.
	if out.Status != component.StatusCompleted || len(out.Requirements) != 0 || out.Content != "final: keep it" {
		t.Fatalf("continued: %+v", out)
	}
	if len(files.Deleted()) != 0 {
		t.Fatal("rejected call must not run")
	}
}

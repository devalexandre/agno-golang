package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// streamingToolModel streams the toolCallingModel answer through the
// streaming callback, executing tools mid-stream like the real clients.
type streamingToolModel struct{ toolCallingModel }

func (m *streamingToolModel) InvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) error {
	resp, err := m.Invoke(ctx, msgs, opts...)
	if err != nil {
		return err
	}
	co := &models.CallOptions{}
	for _, o := range opts {
		o(co)
	}
	if co.StreamingFunc != nil {
		for _, w := range strings.SplitAfter(resp.Content, " ") {
			if err := co.StreamingFunc(ctx, []byte(w)); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestRunStreamPausesWithoutSession(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithConfirmation()}})
	m := &streamingToolModel{toolCallingModel{planned: []tools.FunctionCall{
		call("Ops_Lookup", `{"to":"bob","subject":"x"}`),
		call("Ops_Send", `{"to":"bob","subject":"hi"}`),
	}}}
	a := newHITLAgent(t, m, tool, nil)

	var streamed strings.Builder
	err := a.RunStream("email bob", func(b []byte) error { streamed.Write(b); return nil })
	if !errors.Is(err, ErrRunPaused) {
		t.Fatalf("expected ErrRunPaused, got %v", err)
	}
	var paused *RunPausedError
	if !errors.As(err, &paused) {
		t.Fatalf("expected *RunPausedError, got %T", err)
	}
	resp := paused.Response
	if resp.Status != models.RunStatusPaused || resp.RunID == "" || len(resp.ActiveRequirements()) != 1 {
		t.Fatalf("bad paused response: status=%s run=%q reqs=%d", resp.Status, resp.RunID, len(resp.ActiveRequirements()))
	}
	if streamed.Len() != 0 {
		t.Fatalf("nothing should stream before the pause, got %q", streamed.String())
	}
	if got := tool.called(); len(got) != 1 || got[0] != "lookup:bob" {
		t.Fatalf("only lookup should have run, got %v", got)
	}

	if err := resp.ActiveRequirements()[0].Confirm(); err != nil {
		t.Fatal(err)
	}
	out, err := a.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != models.RunStatusCompleted || !strings.Contains(out.TextContent, "sent to bob") {
		t.Fatalf("expected completed run with send result, got %s %q", out.Status, out.TextContent)
	}
	if got := tool.called(); len(got) != 2 || got[1] != "send:bob:hi" {
		t.Fatalf("send should run once after confirmation, got %v", got)
	}
}

func TestRunStreamWithResponseCompletes(t *testing.T) {
	tool := newOpsTool(nil)
	m := &streamingToolModel{toolCallingModel{planned: []tools.FunctionCall{
		call("Ops_Lookup", `{"to":"amy","subject":"x"}`),
	}}}
	a := newHITLAgent(t, m, tool, nil)

	var streamed strings.Builder
	resp, err := a.RunStreamWithResponse("find amy", func(b []byte) error { streamed.Write(b); return nil },
		WithSessionID("s-stream"), WithRunID("run-1"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != models.RunStatusCompleted || resp.RunID != "run-1" || resp.SessionID != "s-stream" {
		t.Fatalf("bad response: %s %q %q", resp.Status, resp.RunID, resp.SessionID)
	}
	if resp.TextContent != streamed.String() || !strings.Contains(resp.TextContent, "found amy") {
		t.Fatalf("content %q should equal streamed %q", resp.TextContent, streamed.String())
	}
	if err := a.RunStream("find amy", nil); err != nil {
		t.Fatalf("RunStream should succeed: %v", err)
	}
}

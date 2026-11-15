package agui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/os/component"
)

func testComp() *component.Func {
	return &component.Func{
		IDValue: "a", KindValue: component.KindAgent,
		StreamFunc: func(_ context.Context, req component.RunRequest, emit func(component.Event) error) (*component.RunResult, error) {
			if req.Input == "delete it" {
				return &component.RunResult{Status: component.StatusPaused, Requirements: []map[string]any{{
					"pause_type": "confirmation", "tool_execution": map[string]any{"tool_call_id": "call-1", "tool_name": "delete", "tool_args": map[string]any{"path": "/x"}, "requires_confirmation": true}}}}, nil
			}
			_ = emit(component.Event{Type: component.EventRunContent, Content: "Hi "})
			_ = emit(component.Event{Type: component.EventToolCallStarted, Tool: &component.ToolCall{ID: "t1", Name: "lookup", Args: map[string]any{"q": 1}}})
			_ = emit(component.Event{Type: component.EventToolCallCompleted, Tool: &component.ToolCall{ID: "t1", Name: "lookup", Result: "found"}})
			_ = emit(component.Event{Type: component.EventRunContent, Content: req.Input})
			return &component.RunResult{Status: component.StatusCompleted, Content: "Hi " + req.Input}, nil
		},
		ContinueFunc: func(_ context.Context, req component.ContinueRequest) (*component.RunResult, error) {
			te := req.Requirements[0]["tool_execution"].(map[string]any)
			if te["confirmed"] == true {
				return &component.RunResult{Status: component.StatusCompleted, Content: "deleted"}, nil
			}
			return &component.RunResult{Status: component.StatusCompleted, Content: "kept"}, nil
		},
	}
}

func post(t *testing.T, url string, in RunAgentInput) []map[string]any {
	t.Helper()
	b, _ := json.Marshal(in)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var events []map[string]any
	_ = a2a.ReadSSE(resp.Body, func(data []byte) error {
		var ev map[string]any
		_ = json.Unmarshal(data, &ev)
		events = append(events, ev)
		return nil
	})
	return events
}

func types(evs []map[string]any) string {
	var out []string
	for _, e := range evs {
		out = append(out, e["type"].(string))
	}
	return strings.Join(out, ",")
}

func userMsg(text string) Message {
	c, _ := json.Marshal(text)
	return Message{ID: "m", Role: "user", Content: c}
}

func TestRunAndResume(t *testing.T) {
	srv, _ := New(testComp(), &Config{Prefix: "/chat"})
	ts := httptest.NewServer(srv)
	defer ts.Close()

	if r, _ := http.Get(ts.URL + "/chat/status"); r.StatusCode != 200 {
		t.Fatalf("status: %d", r.StatusCode)
	}
	evs := post(t, ts.URL+"/chat/agui", RunAgentInput{ThreadID: "th", Messages: []Message{userMsg("there")}, State: map[string]any{"k": 1}})
	want := "RUN_STARTED,STATE_SNAPSHOT,TEXT_MESSAGE_START,TEXT_MESSAGE_CONTENT,TEXT_MESSAGE_END,TOOL_CALL_START,TOOL_CALL_ARGS,TOOL_CALL_END,TOOL_CALL_RESULT,TEXT_MESSAGE_START,TEXT_MESSAGE_CONTENT,TEXT_MESSAGE_END,STATE_SNAPSHOT,RUN_FINISHED"
	if got := types(evs); got != want {
		t.Fatalf("events:\n got %s\nwant %s", got, want)
	}

	evs = post(t, ts.URL+"/chat/agui", RunAgentInput{ThreadID: "th2", Messages: []Message{userMsg("delete it")}})
	if got := types(evs); got != "RUN_STARTED,TEXT_MESSAGE_START,TEXT_MESSAGE_END,TOOL_CALL_START,TOOL_CALL_ARGS,TOOL_CALL_END,RUN_FINISHED" {
		t.Fatalf("paused events: %s", got)
	}
	if evs[3]["toolCallId"] != "call-1" || evs[4]["delta"] != `{"path":"/x"}` {
		t.Fatalf("tool call: %v %v", evs[3], evs[4])
	}

	answer, _ := json.Marshal(`{"accepted": true}`)
	evs = post(t, ts.URL+"/chat/agui", RunAgentInput{ThreadID: "th2", Messages: []Message{userMsg("delete it"), {ID: "t", Role: "tool", ToolCallID: "call-1", Content: answer}}})
	if types(evs) != "RUN_STARTED,TEXT_MESSAGE_START,TEXT_MESSAGE_CONTENT,TEXT_MESSAGE_END,RUN_FINISHED" || evs[2]["delta"] != "deleted" {
		t.Fatalf("resume: %v", evs)
	}
	// Resuming a thread with nothing paused is an error.
	evs = post(t, ts.URL+"/chat/agui", RunAgentInput{ThreadID: "th2", Messages: []Message{{ID: "t", Role: "tool", ToolCallID: "call-1", Content: answer}}})
	if !strings.HasSuffix(types(evs), runErrorSuffix) {
		t.Fatalf("no paused run: %s", types(evs))
	}
}

const runErrorSuffix = "RUN_ERROR"

func TestResolveRequirementsPartial(t *testing.T) {
	reqs := []map[string]any{
		{"tool_execution": map[string]any{"tool_call_id": "a", "external_execution_required": true}},
		{"tool_execution": map[string]any{"tool_call_id": "b", "external_execution_required": true}},
	}
	c, _ := json.Marshal("result")
	if _, err := ResolveRequirements(reqs, []Message{{Role: "tool", ToolCallID: "a", Content: c}}); err == nil {
		t.Fatal("partial resume must fail")
	}
	out, err := ResolveRequirements(reqs[:1], []Message{{Role: "tool", ToolCallID: "a", Content: c}})
	if err != nil || out[0]["external_execution_result"] != "result" || out[0]["tool_execution"].(map[string]any)["result"] != "result" {
		t.Fatalf("external execution: %v %v", out, err)
	}
	if _, ok := reqs[0]["tool_execution"].(map[string]any)["result"]; ok {
		t.Fatal("input requirements must not be mutated")
	}
}

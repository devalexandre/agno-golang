package a2aserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/os/component"
)

func testAgent() *component.Func {
	return &component.Func{
		IDValue: "helper", NameValue: "Helper", DescriptionValue: "Helps", KindValue: component.KindAgent,
		StreamFunc: func(ctx context.Context, req component.RunRequest, emit func(component.Event) error) (*component.RunResult, error) {
			_ = emit(component.Event{Type: component.EventRunStarted, RunID: req.RunID})
			switch req.Input {
			case "pause":
				return &component.RunResult{RunID: req.RunID, Status: component.StatusPaused,
					Requirements: []map[string]any{{"id": "req1", "tool_execution": map[string]any{"tool_call_id": "call1", "tool_name": "delete_file"}}}}, nil
			case "slow":
				<-ctx.Done()
				return nil, ctx.Err()
			}
			for _, chunk := range []string{"Hello ", req.Input} {
				_ = emit(component.Event{Type: component.EventRunContent, Content: chunk})
			}
			return &component.RunResult{RunID: req.RunID, Status: component.StatusCompleted, Content: "Hello " + req.Input}, nil
		},
		ContinueFunc: func(_ context.Context, req component.ContinueRequest) (*component.RunResult, error) {
			if req.Requirements[0]["confirmation"] != true {
				return &component.RunResult{RunID: req.RunID, Status: component.StatusCompleted, Content: "rejected"}, nil
			}
			return &component.RunResult{RunID: req.RunID, Status: component.StatusCompleted, Content: "deleted"}, nil
		},
	}
}

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	srv, err := New(component.NewRegistry(testAgent()), nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts, srv
}

func rpc(t *testing.T, url, method string, params any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func TestAgentCard(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/a2a/agents/helper/.well-known/agent-card.json")
	if err != nil {
		t.Fatal(err)
	}
	var card a2a.AgentCard
	_ = json.NewDecoder(resp.Body).Decode(&card)
	resp.Body.Close()
	if card.Name != "Helper" || len(card.SupportedInterfaces) != 2 || card.URL != ts.URL+"/a2a/agents/helper" || !card.Capabilities.Streaming {
		t.Fatalf("card: %+v", card)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/a2a/.well-known/agent-card.json", nil)
	req.Header.Set("If-None-Match", resp.Header.Get("ETag"))
	resp2, _ := http.DefaultClient.Do(req)
	if resp2.StatusCode != http.StatusNotModified {
		t.Fatalf("etag: %d", resp2.StatusCode)
	}
	if r, _ := http.Get(ts.URL + "/a2a/agents/nope/.well-known/agent-card.json"); r.StatusCode != 404 {
		t.Fatalf("missing: %d", r.StatusCode)
	}
}

func TestSendV1AndV03(t *testing.T) {
	ts, _ := newTestServer(t)
	url := ts.URL + "/a2a/agents/helper"

	out := rpc(t, url, a2a.MethodSendMessage, map[string]any{"message": map[string]any{
		"messageId": "m1", "role": "ROLE_USER", "parts": []any{map[string]any{"text": "world"}}}})
	task := out["result"].(map[string]any)["task"].(map[string]any)
	if task["status"].(map[string]any)["state"] != "TASK_STATE_COMPLETED" {
		t.Fatalf("v1 result: %v", out)
	}
	art := task["artifacts"].([]any)[0].(map[string]any)
	if art["parts"].([]any)[0].(map[string]any)["text"] != "Hello world" {
		t.Fatalf("artifact: %v", art)
	}

	out = rpc(t, url, a2a.MethodV03Send, map[string]any{"message": map[string]any{
		"kind": "message", "messageId": "m2", "role": "user", "parts": []any{map[string]any{"kind": "text", "text": "v03"}}}})
	res := out["result"].(map[string]any)
	if res["kind"] != "task" || res["status"].(map[string]any)["state"] != "completed" {
		t.Fatalf("v0.3 result: %v", out)
	}
	got := rpc(t, url, a2a.MethodV03GetTask, map[string]any{"id": res["id"]})
	if got["result"].(map[string]any)["id"] != res["id"] {
		t.Fatalf("get: %v", got)
	}
	if e := rpc(t, url, a2a.MethodGetTask, map[string]any{"id": "missing"})["error"].(map[string]any); e["code"].(float64) != a2a.CodeTaskNotFound {
		t.Fatalf("not found: %v", e)
	}
	if e := rpc(t, url, "Bogus", map[string]any{})["error"].(map[string]any); e["code"].(float64) != a2a.CodeMethodNotFound {
		t.Fatalf("method: %v", e)
	}
	list := rpc(t, url, a2a.MethodListTasks, map[string]any{})["result"].(map[string]any)
	if list["totalSize"].(float64) != 2 {
		t.Fatalf("list: %v", list)
	}
}

func TestInputRequiredAndContinue(t *testing.T) {
	ts, _ := newTestServer(t)
	c := a2a.NewClient(ts.URL + "/a2a/agents/helper")
	ctx := context.Background()
	res, err := c.SendMessage(ctx, "pause", a2a.SendOptions{UserID: "u1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsInputRequired() || !strings.Contains(res.Content, "delete_file") {
		t.Fatalf("paused: %+v", res)
	}
	reqs := res.Data.(map[string]any)["requirements"].([]any)
	// Unknown requirement: refused, the task keeps waiting.
	bad, err := c.SendMessage(ctx, "", a2a.SendOptions{TaskID: res.TaskID, Data: map[string]any{"requirements": []any{map[string]any{"id": "other"}}}})
	if err != nil || !bad.IsInputRequired() || !strings.Contains(bad.Content, "not the ones") {
		t.Fatalf("bad continue: %+v %v", bad, err)
	}
	reqs[0].(map[string]any)["confirmation"] = true
	done, err := c.SendMessage(ctx, "", a2a.SendOptions{TaskID: res.TaskID, Data: map[string]any{"requirements": reqs}})
	if err != nil || !done.IsCompleted() || done.Content != "deleted" {
		t.Fatalf("continue: %+v %v", done, err)
	}
	if _, err := c.SendMessage(ctx, "again", a2a.SendOptions{TaskID: res.TaskID}); err == nil {
		t.Fatal("terminal task must refuse messages")
	}
}

func TestStreamingClient(t *testing.T) {
	ts, _ := newTestServer(t)
	c := a2a.NewClient(ts.URL + "/a2a/agents/helper")
	var types []string
	res, err := c.StreamMessage(context.Background(), "stream", a2a.SendOptions{}, func(ev a2a.StreamEvent) error {
		types = append(types, ev.EventType)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(types, ",")
	if !strings.HasPrefix(joined, "started,working,content,content") || !strings.HasSuffix(joined, "completed") {
		t.Fatalf("events: %s", joined)
	}
	if res.Content != "Hello stream" || !res.IsCompleted() {
		t.Fatalf("stream result: %+v", res)
	}
}

func TestCancelNonBlocking(t *testing.T) {
	ts, _ := newTestServer(t)
	url := ts.URL + "/a2a/agents/helper"
	out := rpc(t, url, a2a.MethodSendMessage, map[string]any{
		"message":       map[string]any{"messageId": "m", "role": "ROLE_USER", "parts": []any{map[string]any{"text": "slow"}}},
		"configuration": map[string]any{"returnImmediately": true}})
	task := out["result"].(map[string]any)["task"].(map[string]any)
	if task["status"].(map[string]any)["state"] != "TASK_STATE_WORKING" {
		t.Fatalf("non-blocking: %v", task)
	}
	id := task["id"]
	canceled := rpc(t, url, a2a.MethodCancelTask, map[string]any{"id": id})["result"].(map[string]any)
	if canceled["status"].(map[string]any)["state"] != "TASK_STATE_CANCELED" {
		t.Fatalf("cancel: %v", canceled)
	}
	time.Sleep(50 * time.Millisecond)
	got := rpc(t, url, a2a.MethodGetTask, map[string]any{"id": id})["result"].(map[string]any)
	if got["status"].(map[string]any)["state"] != "TASK_STATE_CANCELED" {
		t.Fatalf("state after cancel: %v", got)
	}
	if e := rpc(t, url, a2a.MethodCancelTask, map[string]any{"id": id})["error"].(map[string]any); e["code"].(float64) != a2a.CodeTaskNotCancelable {
		t.Fatalf("not cancelable: %v", e)
	}
}

func TestRemoteAgentComponent(t *testing.T) {
	ts, _ := newTestServer(t)
	remote := a2a.NewRemoteAgent(a2a.NewClient(ts.URL+"/a2a/agents/helper"), "")
	if err := remote.Refresh(context.Background()); err != nil || remote.Name() != "Helper" {
		t.Fatalf("refresh: %v %s", err, remote.Name())
	}
	res, err := remote.Run(context.Background(), component.RunRequest{Input: "remote", SessionID: "ctx1"})
	if err != nil || res.Content != "Hello remote" || res.SessionID != "ctx1" {
		t.Fatalf("remote run: %+v %v", res, err)
	}
	tool := remote.AsTool("", "")
	out, err := tool.Ask(a2a.AskParams{Message: "tool"})
	if err != nil || !strings.Contains(out.(string), "Hello tool") {
		t.Fatalf("tool: %v %v", out, err)
	}
	paused, _ := remote.Run(context.Background(), component.RunRequest{Input: "pause"})
	if paused.Status != component.StatusPaused || len(paused.Requirements) != 1 {
		t.Fatalf("remote paused: %+v", paused)
	}
}

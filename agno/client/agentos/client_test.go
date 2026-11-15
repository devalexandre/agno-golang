package agentos

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devalexandre/agno-golang/agno/os/component"
)

func fakeOS(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /agents", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":"agent_1","name":"Helper","description":"helps"}]`)
	})
	mux.HandleFunc("GET /agents/agent_1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"agent_1","name":"Helper","description":"helps"}`)
	})
	mux.HandleFunc("POST /agents/agent_1/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			w.WriteHeader(401)
			return
		}
		_ = r.ParseForm()
		msg, sid := r.PostForm.Get("message"), r.PostForm.Get("session_id")
		if r.PostForm.Get("stream") == "true" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "event: RunStarted\ndata: {\"event\":\"RunStarted\",\"run_id\":\"r1\",\"session_id\":%q}\n\n", sid)
			fmt.Fprint(w, "event: RunContent\ndata: {\"event\":\"RunContent\",\"run_id\":\"r1\",\"content\":\"Hello \"}\n\n")
			fmt.Fprintf(w, "event: RunContent\ndata: {\"event\":\"RunContent\",\"run_id\":\"r1\",\"content\":%q}\n\n", msg)
			fmt.Fprint(w, "event: RunCompleted\ndata: {\"event\":\"RunCompleted\",\"run_id\":\"r1\"}\n\n")
			return
		}
		fmt.Fprintf(w, `{"run_id":"r2","session_id":%q,"content":"Hello %s","status":"COMPLETED"}`, sid, msg)
	})
	mux.HandleFunc("POST /agents/agent_1/runs/r1/cancel", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) })
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestClientAndRemoteComponent(t *testing.T) {
	ts := fakeOS(t)
	c := New(ts.URL, "key")
	ctx := context.Background()
	agents, err := c.List(ctx, component.KindAgent)
	if err != nil || len(agents) != 1 || agents[0].ID != "agent_1" {
		t.Fatalf("list: %v %v", agents, err)
	}
	out, err := c.Run(ctx, component.KindAgent, "agent_1", RunParams{Message: "go", SessionID: "s1"})
	if err != nil || out.Text() != "Hello go" || out.SessionID != "s1" {
		t.Fatalf("run: %+v %v", out, err)
	}
	remote := NewRemote(c, component.KindAgent, "agent_1")
	if err := remote.Refresh(ctx); err != nil || remote.Name() != "Helper" {
		t.Fatalf("refresh: %v", err)
	}
	var chunks []string
	res, err := component.Stream(ctx, remote, component.RunRequest{Input: "stream", SessionID: "s2"}, func(ev component.Event) error {
		if ev.Type == component.EventRunContent {
			chunks = append(chunks, ev.Content)
		}
		return nil
	})
	if err != nil || res.Content != "Hello stream" || len(chunks) != 2 || res.SessionID != "s2" || res.Status != component.StatusCompleted {
		t.Fatalf("stream: %+v %v %v", res, chunks, err)
	}
	if ok, err := remote.CancelRun(ctx, "r1"); !ok || err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := New(ts.URL, "").Run(ctx, component.KindAgent, "agent_1", RunParams{Message: "x"}); err == nil {
		t.Fatal("expected 401")
	}
}

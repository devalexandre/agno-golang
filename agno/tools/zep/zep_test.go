package zep

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestZepFlow(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var msgBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Header.Get("Authorization") != "Api-Key k" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/users/alice":
			w.WriteHeader(404)
		case r.URL.Path == "/users", r.URL.Path == "/threads":
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/messages") && r.Method == "POST":
			_ = json.NewDecoder(r.Body).Decode(&msgBody)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/context"):
			_, _ = w.Write([]byte(`{"context":"Alice likes Go"}`))
		case strings.HasSuffix(r.URL.Path, "/messages"):
			_, _ = w.Write([]byte(`{"messages":[]}`))
		case r.URL.Path == "/graph/search":
			_, _ = w.Write([]byte(`{"edges":[{"fact":"Alice likes Go"}],"nodes":[]}`))
		}
	}))
	defer srv.Close()

	z := NewZepTools(WithZepAPIKey("k"), WithZepBaseURL(srv.URL), WithZepUserID("alice"), WithZepSessionID("s1"), WithZepIgnoreAssistantMessages(true))
	out, _ := z.AddZepMessage(AddZepMessageParams{Role: "user", Content: "I like Go"})
	if out != "Message from 'user' added successfully to session s1." {
		t.Fatalf("add: %v (calls %v)", out, calls)
	}
	if calls[0] != "GET /users/alice" || calls[1] != "POST /users" || calls[2] != "POST /threads" {
		t.Fatalf("init calls %v", calls)
	}
	if msgBody["ignore_roles"] == nil {
		t.Fatal("ignore_roles missing")
	}
	if out, _ = z.GetZepMemory(GetZepMemoryParams{}); out != "Alice likes Go" {
		t.Fatalf("context %v", out)
	}
	if out, _ = z.GetZepMemory(GetZepMemoryParams{MemoryType: "messages"}); out != "No messages available." {
		t.Fatalf("messages %v", out)
	}
	if out, _ = z.SearchZepMemory(SearchZepMemoryParams{Query: "go"}); out != "Found 1 facts:\n- Alice likes Go" {
		t.Fatalf("search %v", out)
	}
	if out, _ = z.SearchZepMemory(SearchZepMemoryParams{Query: "go", SearchScope: "nodes"}); out != "No nodes found for query: go" {
		t.Fatalf("search nodes %v", out)
	}
}

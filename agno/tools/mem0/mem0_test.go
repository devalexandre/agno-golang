package mem0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMem0Flow(t *testing.T) {
	var lastBody map[string]interface{}
	var lastQuery, lastAuth, lastMethod, lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth, lastMethod, lastPath, lastQuery = r.Header.Get("Authorization"), r.Method, r.URL.Path, r.URL.RawQuery
		lastBody = nil
		_ = json.NewDecoder(r.Body).Decode(&lastBody)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/memories/":
			_, _ = w.Write([]byte(`{"results":[{"id":"m1","event":"ADD"}]}`))
		case r.URL.Path == "/v1/memories/search/":
			_, _ = w.Write([]byte(`{"results":[{"memory":"lives in NYC"}]}`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{"memory":"a"},{"memory":"b"}]`))
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"message":"ok"}`))
		}
	}))
	defer srv.Close()

	m := NewMem0Tools(WithMem0APIKey("k"), WithMem0BaseURL(srv.URL), WithMem0UserID("u1"), WithMem0OrgID("o"))
	if len(m.GetMethods()) != 4 {
		t.Fatalf("methods: %d", len(m.GetMethods()))
	}
	if !m.GetMethods()["mem0_tools_DeleteAllMemories"].RequiresConfirmation {
		t.Fatal("delete should require confirmation")
	}

	out, _ := m.AddMemory(AddMemoryParams{Content: map[string]interface{}{"Name": "John"}})
	if lastAuth != "Token k" || lastBody["user_id"] != "u1" || lastBody["org_id"] != "o" || lastBody["infer"] != true {
		t.Fatalf("bad add request: %v %v", lastAuth, lastBody)
	}
	msgs := lastBody["messages"].([]interface{})
	if msgs[0].(map[string]interface{})["content"] != `{"Name":"John"}` {
		t.Fatalf("content not json-wrapped: %v", msgs)
	}
	if !strings.Contains(out.(string), "m1") {
		t.Fatalf("add output %v", out)
	}

	out, _ = m.SearchMemory(SearchMemoryParams{Query: "where"})
	if out.(string) != `[{"memory":"lives in NYC"}]` {
		t.Fatalf("search output %v", out)
	}
	out, _ = m.GetAllMemories(NoParams{})
	if lastMethod != http.MethodGet || !strings.Contains(lastQuery, "user_id=u1") || out.(string) != `[{"memory":"a"},{"memory":"b"}]` {
		t.Fatalf("get all %v %v", lastQuery, out)
	}
	out, _ = m.DeleteAllMemories(NoParams{})
	if lastMethod != http.MethodDelete || lastPath != "/v1/memories/" || !strings.Contains(out.(string), "Successfully deleted") {
		t.Fatalf("delete %v", out)
	}
}

func TestMem0RequiresUser(t *testing.T) {
	m := NewMem0Tools(WithMem0APIKey("k"))
	out, _ := m.SearchMemory(SearchMemoryParams{Query: "x"})
	if !strings.HasPrefix(out.(string), "Error in search_memory") {
		t.Fatalf("got %v", out)
	}
}

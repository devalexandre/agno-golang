package memorytools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/memory"
)

type fakeDB struct{ mems map[string]*memory.UserMemory }

func (f *fakeDB) CreateUserMemory(_ context.Context, m *memory.UserMemory) error {
	f.mems[m.ID] = m
	return nil
}
func (f *fakeDB) GetUserMemories(_ context.Context, userID string) ([]*memory.UserMemory, error) {
	var out []*memory.UserMemory
	for _, m := range f.mems {
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (f *fakeDB) UpdateUserMemory(_ context.Context, m *memory.UserMemory) error {
	f.mems[m.ID] = m
	return nil
}
func (f *fakeDB) DeleteUserMemory(_ context.Context, id string) error {
	delete(f.mems, id)
	return nil
}

func TestMemoryTools(t *testing.T) {
	db := &fakeDB{mems: map[string]*memory.UserMemory{}}
	m, err := NewMemoryTools(db, WithUserID("u1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.GetMethods()) != 6 {
		t.Fatalf("expected 6 tools, got %d", len(m.GetMethods()))
	}
	out, _ := m.AddMemory(AddMemoryParams{Memory: "likes Go", Topics: []string{"lang"}})
	var added struct {
		Success bool
		Memory  map[string]interface{}
	}
	if err := json.Unmarshal([]byte(out.(string)), &added); err != nil || !added.Success {
		t.Fatalf("add: %v", out)
	}
	id := added.Memory["memory_id"].(string)

	if out, _ := m.GetMemories(GetMemoriesParams{}); !strings.Contains(out.(string), "likes Go") {
		t.Fatalf("get: %v", out)
	}
	if out, _ := m.UpdateMemory(UpdateMemoryParams{MemoryID: id, Memory: "loves Go"}); !strings.Contains(out.(string), "loves Go") {
		t.Fatalf("update: %v", out)
	}
	if out, _ := m.UpdateMemory(UpdateMemoryParams{MemoryID: "nope"}); !strings.Contains(out.(string), "not found") {
		t.Fatalf("update missing: %v", out)
	}
	if out, _ := m.DeleteMemory(DeleteMemoryParams{MemoryID: id}); !strings.Contains(out.(string), `"success": true`) {
		t.Fatalf("delete: %v", out)
	}
	if len(db.mems) != 0 {
		t.Fatal("memory not deleted")
	}
	if out, _ := m.Think(ThinkParams{Thought: "t"}); out != "Memory Thoughts:\n- t" {
		t.Fatalf("think: %v", out)
	}
	ops := m.SessionState()["memory_operations"].([]interface{})
	if len(ops) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(ops))
	}
}

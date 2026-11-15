package jsonstore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func TestJSONConformance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	db, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	storagetest.RunDBConformance(t, db)
}

func TestJSONPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "store.json")

	db, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sess := storage.NewAgentSession("s1", "u1", "a")
	sess.Memory["greeting"] = "hi"
	if err := db.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Re-open from disk and verify the data survived.
	db2, err := New(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := db2.ReadSession(ctx, "s1")
	if err != nil || got == nil {
		t.Fatalf("ReadSession after reopen: got=%v err=%v", got, err)
	}
	if got.Memory["greeting"] != "hi" {
		t.Fatalf("data not persisted across reopen: %+v", got.Memory)
	}
}

package inmemory

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func TestInMemoryConformance(t *testing.T) {
	storagetest.RunDBConformance(t, New())
	storagetest.RunSessionSearchConformance(t, New())
	storagetest.RunMetricsConformance(t, New())
}

func TestInMemoryDuplicateErrors(t *testing.T) {
	ctx := context.Background()
	db := New()
	s := storage.NewAgentSession("dup", "u", "a")
	if err := db.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(ctx, s); err == nil {
		t.Fatal("expected duplicate session error")
	}
}

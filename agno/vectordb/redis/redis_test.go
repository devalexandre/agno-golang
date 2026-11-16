package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

func startRedisStack(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "redis/redis-stack-server:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Skipf("skipping: cannot start redis-stack container (docker unavailable?): %v", err)
	}
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379")
	return host + ":" + port.Port(), func() { _ = c.Terminate(ctx) }
}

func TestRedisIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ctx := context.Background()
	addr, cleanup := startRedisStack(ctx, t)
	defer cleanup()

	db := New(Options{Addr: addr, Embedder: testembed.New(32)})
	defer db.Close()

	if err := db.Create(ctx); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ok, _ := db.Exists(ctx); !ok {
		t.Fatal("index should exist after Create")
	}

	docs := []*document.Document{
		{ID: "1", Content: "the cat sat on the mat", Name: "a"},
		{ID: "2", Content: "quantum physics and relativity", Name: "b"},
		{ID: "3", Content: "a happy dog runs in the park", Name: "c"},
	}
	if err := db.Insert(ctx, docs, nil); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if n, _ := db.GetCount(ctx); n != 3 {
		t.Fatalf("GetCount = %d, want 3", n)
	}
	if ok, _ := db.IDExists(ctx, "2"); !ok {
		t.Fatal("IDExists(2) should be true")
	}

	// Searching for an exact stored content should rank that document first
	// (deterministic embedder => identical vector).
	res, err := db.VectorSearch(ctx, "quantum physics and relativity", 3, nil)
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected search results")
	}
	if !strings.Contains(res[0].Document.Content, "quantum") {
		t.Fatalf("expected quantum doc first, got %q", res[0].Document.Content)
	}
}

func TestRedisConstruct(t *testing.T) {
	db := New(Options{Addr: "localhost:6379", Embedder: testembed.New(8)})
	var _ vectordb.VectorDB = db
	if db.dimension != 8 {
		t.Fatalf("dimension = %d", db.dimension)
	}
}

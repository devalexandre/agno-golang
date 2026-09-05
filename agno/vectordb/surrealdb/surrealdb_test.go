package surrealdb

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

func startSurreal(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "surrealdb/surrealdb:latest",
		ExposedPorts: []string{"8000/tcp"},
		Cmd:          []string{"start", "--user", "root", "--pass", "root", "--bind", "0.0.0.0:8000", "memory"},
		WaitingFor:   wait.ForHTTP("/health").WithPort("8000/tcp").WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Skipf("skipping: cannot start surrealdb container: %v", err)
	}
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "8000")
	return "http://" + host + ":" + port.Port(), func() { _ = c.Terminate(ctx) }
}

func TestSurrealIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ctx := context.Background()
	url, cleanup := startSurreal(ctx, t)
	defer cleanup()

	db := New(Options{
		BaseURL: url, User: "root", Pass: "root",
		Namespace: "test", Database: "test",
		Embedder: testembed.New(32),
	})

	if err := db.Create(ctx); err != nil {
		t.Fatalf("Create: %v", err)
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

	res, err := db.VectorSearch(ctx, "quantum physics and relativity", 3, nil)
	if err != nil {
		t.Fatalf("VectorSearch: %v", err)
	}
	if len(res) == 0 || !strings.Contains(res[0].Document.Content, "quantum") {
		t.Fatalf("expected quantum doc first, got %+v", res)
	}
}

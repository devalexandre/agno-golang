package clickhouse

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

func startClickHouse(ctx context.Context, t *testing.T) (string, func()) {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env:          map[string]string{"CLICKHOUSE_PASSWORD": "test", "CLICKHOUSE_USER": "default"},
		WaitingFor:   wait.ForListeningPort("9000/tcp").WithStartupTimeout(120 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Skipf("skipping: cannot start clickhouse container: %v", err)
	}
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "9000")
	return host + ":" + port.Port(), func() { _ = c.Terminate(ctx) }
}

func TestClickHouseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	ctx := context.Background()
	addr, cleanup := startClickHouse(ctx, t)
	defer cleanup()

	db, err := New(Options{Addr: []string{addr}, Password: "test", Embedder: testembed.New(32)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	if err := db.Create(ctx); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ok, _ := db.Exists(ctx); !ok {
		t.Fatal("table should exist after Create")
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

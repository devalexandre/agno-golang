package clickhouse

import (
	"os"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func TestListQuery(t *testing.T) {
	b := &Backend{table: "agno_documents"}
	q, args := b.ListQuery("runs", map[string]string{"session_id": "s1"})
	if !strings.Contains(q, "argMax(idx, version)[?] = ?") || len(args) != 3 || args[1] != "session_id" {
		t.Fatalf("unexpected query %q %v", q, args)
	}
	if !strings.Contains(b.CreateTableSQL(), "ReplacingMergeTree(version)") {
		t.Fatal("unexpected DDL")
	}
	if v1, v2 := b.version(), b.version(); v2 <= v1 {
		t.Fatal("versions must increase")
	}
}

// TestClickHouseRealServer runs when CLICKHOUSE_DSN is set, e.g.
// docker run -p 9000:9000 clickhouse/clickhouse-server
// CLICKHOUSE_DSN=clickhouse://default:@localhost:9000/default
func TestClickHouseRealServer(t *testing.T) {
	if os.Getenv("CLICKHOUSE_DSN") == "" {
		t.Skip("CLICKHOUSE_DSN not set")
	}
	s, err := New(Config{Prefix: "agno_conformance"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateTables(t.Context()); err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(t.Context())
	defer s.DropTables(t.Context())
	storagetest.RunDBConformance(t, s)
	_ = s.DropTables(t.Context())
	storagetest.RunSessionSearchConformance(t, s)
	_ = s.DropTables(t.Context())
	storagetest.RunMetricsConformance(t, s)
}

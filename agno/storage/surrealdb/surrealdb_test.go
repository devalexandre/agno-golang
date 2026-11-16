package surrealdb

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func TestQueries(t *testing.T) {
	b, _ := NewBackend(Config{URL: "ws://localhost:8000/rpc"})
	if b.cfg.URL != "http://localhost:8000" {
		t.Fatalf("url normalization: %s", b.cfg.URL)
	}
	q := b.PutQuery("sessions", `a"b`, []byte(`{"k":"v"}`), map[string]string{"user_id": "u"})
	if !strings.HasPrefix(q, `UPSERT type::thing("agno_sessions", "a\"b") CONTENT {`) || !strings.Contains(q, `"data":"{\"k\":\"v\"}"`) {
		t.Fatalf("unexpected put query: %s", q)
	}
	l := b.ListQuery("runs", map[string]string{"session_id": "s1"})
	if l != `SELECT data, id FROM type::table("agno_runs") WHERE idx["session_id"] = "s1" ORDER BY id;` {
		t.Fatalf("unexpected list query: %s", l)
	}
}

// TestSurrealRealServer runs when SURREALDB_URL is set, e.g.
// docker run -p 8000:8000 surrealdb/surrealdb:latest start --user root --pass root
// SURREALDB_URL=http://localhost:8000 SURREALDB_USER=root SURREALDB_PASS=root
func TestSurrealRealServer(t *testing.T) {
	if os.Getenv("SURREALDB_URL") == "" {
		t.Skip("SURREALDB_URL not set")
	}
	ctx := context.Background()
	s, err := New(Config{Prefix: "agno_conformance"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(ctx)
	defer s.DropTables(ctx)
	storagetest.RunDBConformance(t, s)
	_ = s.DropTables(ctx)
	storagetest.RunSessionSearchConformance(t, s)
	_ = s.DropTables(ctx)
	storagetest.RunMetricsConformance(t, s)
}

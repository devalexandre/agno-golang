package sqlstore

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

// SQLite uses "?" placeholders like MySQL and accepts the portable DDL, so it
// exercises the exact shared SQL code path that the postgres and mysql backends
// use — offline, with no container required.
func TestSQLStoreConformanceSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := New(db, Dialect{Driver: "sqlite3", Positional: false}, "agno_test")
	storagetest.RunDBConformance(t, store)
}

func newSQLite(t *testing.T) *SQLStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return New(db, Dialect{Driver: "sqlite3"}, "agno_test")
}

func TestSQLStoreExtendedConformanceSQLite(t *testing.T) {
	storagetest.RunSessionSearchConformance(t, newSQLite(t))
	storagetest.RunMetricsConformance(t, newSQLite(t))
	s := newSQLite(t)
	if err := s.CreateTables(t.Context()); err != nil {
		t.Fatal(err)
	}
	storagetest.RunKnowledgeConformance(t, s)
}

func TestRebindPositional(t *testing.T) {
	d := Dialect{Positional: true}
	got := d.rebind("SELECT * FROM t WHERE a = ? AND b = ?")
	want := "SELECT * FROM t WHERE a = $1 AND b = $2"
	if got != want {
		t.Fatalf("rebind = %q, want %q", got, want)
	}

	d2 := Dialect{Positional: false}
	q := "SELECT ? , ?"
	if d2.rebind(q) != q {
		t.Fatalf("non-positional rebind should be a no-op")
	}
}

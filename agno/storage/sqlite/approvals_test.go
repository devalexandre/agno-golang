package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/approval/approvaltest"
)

func TestSqliteApprovalConformance(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "approvals.db")
	db, err := NewSqliteStorage(SqliteStorageConfig{TableName: "agno_sessions", DBFile: &dbFile})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	approvaltest.RunStoreConformance(t, db)

	// Records survive reopening the database file.
	db2, err := NewSqliteStorage(SqliteStorageConfig{TableName: "agno_sessions", DBFile: &dbFile})
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	got, err := db2.GetApproval(context.Background(), "ap_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != approval.StatusApproved || got.ResolvedBy != "alice" {
		t.Fatalf("approval not persisted: %+v", got)
	}
}

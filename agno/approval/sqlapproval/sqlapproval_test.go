package sqlapproval

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/devalexandre/agno-golang/agno/approval/approvaltest"
)

func TestSQLApprovalConformance(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	approvaltest.RunStoreConformance(t, New(db, "agno_approvals", false))
}

func TestRebindPositional(t *testing.T) {
	s := New(nil, "t", true)
	if got := s.rebind("a = ? AND b = ?"); got != "a = $1 AND b = $2" {
		t.Fatalf("rebind = %q", got)
	}
}

package singlestore

import (
	"os"
	"testing"

	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

// TestSingleStoreRealServer runs when SINGLESTORE_DSN is set, e.g.
// docker run -p 3306:3306 -e ROOT_PASSWORD=pw ghcr.io/singlestore-labs/singlestoredb-dev
// SINGLESTORE_DSN='root:pw@tcp(localhost:3306)/agno'
func TestSingleStoreRealServer(t *testing.T) {
	if os.Getenv("SINGLESTORE_DSN") == "" {
		t.Skip("SINGLESTORE_DSN not set")
	}
	s, err := New("", "agno_conformance")
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(t.Context())
	defer s.DropTables(t.Context())
	storagetest.RunDBConformance(t, s)
	_ = s.DropTables(t.Context())
	storagetest.RunSessionSearchConformance(t, s)
}

func TestNewRequiresDSN(t *testing.T) {
	t.Setenv("SINGLESTORE_DSN", "")
	if _, err := New("", ""); err == nil {
		t.Fatal("expected error without DSN")
	}
}

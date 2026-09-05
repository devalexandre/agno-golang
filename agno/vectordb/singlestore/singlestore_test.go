package singlestore

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/devalexandre/agno-golang/agno/vectordb/internal/testembed"
)

func TestConstruct(t *testing.T) {
	db, err := New(Options{DSN: "user:pass@tcp(localhost:3306)/db", Embedder: testembed.New(16)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()
	var _ vectordb.VectorDB = db
	if db.dimension != 16 {
		t.Fatalf("dimension = %d", db.dimension)
	}
}

func TestRequiresDSN(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error when neither DSN nor DB provided")
	}
}

func TestVecJSON(t *testing.T) {
	if got := vecJSON([]float64{1, 2, 3}); got != "[1,2,3]" {
		t.Fatalf("vecJSON = %q", got)
	}
}

package milvus

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

func TestNewDefaults(t *testing.T) {
	db := New(Options{Embedder: embedder.NewMockEmbedder(16)})
	if db.baseURL != "http://localhost:19530" {
		t.Fatalf("default baseURL = %q", db.baseURL)
	}
	if db.collection != "agno_documents" {
		t.Fatalf("default collection = %q", db.collection)
	}
	if db.dimension != 16 {
		t.Fatalf("dimension should default to embedder dims (16), got %d", db.dimension)
	}
	var _ vectordb.VectorDB = db
}

func TestNewExplicitDimension(t *testing.T) {
	db := New(Options{Collection: "c", Dimension: 768})
	if db.dimension != 768 {
		t.Fatalf("dimension = %d, want 768", db.dimension)
	}
}

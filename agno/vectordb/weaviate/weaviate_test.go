package weaviate

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

func TestNewDefaults(t *testing.T) {
	db := New(Options{Embedder: embedder.NewMockEmbedder(8)})
	if db.baseURL != "http://localhost:8080" {
		t.Fatalf("default baseURL = %q", db.baseURL)
	}
	if db.class != "AgnoDocuments" {
		t.Fatalf("default class = %q", db.class)
	}
	var _ vectordb.VectorDB = db
}

func TestToUUID(t *testing.T) {
	// arbitrary string -> deterministic UUID
	a := toUUID("doc-1")
	b := toUUID("doc-1")
	if a != b {
		t.Fatalf("toUUID not deterministic: %q != %q", a, b)
	}
	// already a UUID -> unchanged
	u := "6f9619ff-8b86-d011-b42d-00cf4fc964ff"
	if toUUID(u) != u {
		t.Fatalf("valid UUID should pass through, got %q", toUUID(u))
	}
	if len(a) != 36 {
		t.Fatalf("derived UUID has wrong length: %q", a)
	}
}

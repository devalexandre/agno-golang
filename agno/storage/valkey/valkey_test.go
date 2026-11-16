package valkey

import (
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func TestValkeyConformanceMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	s, err := New(Config{URL: "valkey://" + mr.Addr() + "/0"})
	if err != nil {
		t.Fatal(err)
	}
	storagetest.RunDBConformance(t, s)
}

func TestValkeyRealServer(t *testing.T) {
	if os.Getenv("VALKEY_URL") == "" {
		t.Skip("VALKEY_URL not set")
	}
	s, err := New(Config{Prefix: "agno_test"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(t.Context())
	defer s.DropTables(t.Context())
	storagetest.RunDBConformance(t, s)
}

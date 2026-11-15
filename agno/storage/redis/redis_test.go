package redis

import (
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

func newMini(t *testing.T) *kvstore.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	s, err := New(Config{Client: goredis.NewClient(&goredis.Options{Addr: mr.Addr()})})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRedisConformanceMiniredis(t *testing.T) {
	storagetest.RunDBConformance(t, newMini(t))
	storagetest.RunSessionSearchConformance(t, newMini(t))
	storagetest.RunMetricsConformance(t, newMini(t))
	storagetest.RunKnowledgeConformance(t, newMini(t))
	storagetest.RunStorageConformance(t, newMini(t))
}

func TestRedisDropAndDuplicate(t *testing.T) {
	s := newMini(t)
	storagetest.RunDBConformance(t, s)
	if err := s.DropTables(t.Context()); err != nil {
		t.Fatal(err)
	}
	ids, err := s.GetAllSessionIDs(t.Context(), "")
	if err != nil || len(ids) != 0 {
		t.Fatalf("after drop: %v %v", ids, err)
	}
}

// TestRedisRealServer runs against a real server when REDIS_URL is set.
func TestRedisRealServer(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	s, err := New(Config{URL: url, Prefix: "agno_test"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.DropTables(t.Context())
	defer s.DropTables(t.Context())
	storagetest.RunDBConformance(t, s)
}

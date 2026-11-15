package mongo

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/storage/storagetest"
)

// TestDocumentRoundTrip checks the JSON <-> BSON conversion without a server.
func TestDocumentRoundTrip(t *testing.T) {
	run := &storage.AgentRun{
		ID: "r1", SessionID: "s1", CreatedAt: time.Unix(1700000000, 0).UTC(),
		Messages: []map[string]interface{}{{"role": "user", "content": "hi"}},
		Metrics:  map[string]interface{}{"input_tokens": float64(10), "cost": 0.25},
	}
	raw, _ := json.Marshal(run)
	d, err := toDocument("r1", raw, map[string]string{"session_id": "s1"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := bson.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	back, err := fromRaw(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var got storage.AgentRun
	if err := json.Unmarshal(back, &got); err != nil {
		t.Fatalf("decode %s: %v", back, err)
	}
	if got.ID != "r1" || !got.CreatedAt.Equal(run.CreatedAt) || got.Metrics["cost"] != 0.25 || got.Messages[0]["content"] != "hi" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

// TestMongoRealServer runs the conformance suites when MONGODB_URI is set
// (e.g. docker run -p 27017:27017 mongo:7).
func TestMongoRealServer(t *testing.T) {
	if os.Getenv("MONGODB_URI") == "" {
		t.Skip("MONGODB_URI not set")
	}
	ctx := context.Background()
	mk := func(db string) storagetest.MetricsDB {
		s, err := New(ctx, Config{Database: db})
		if err != nil {
			t.Fatal(err)
		}
		_ = s.DropTables(ctx)
		t.Cleanup(func() { _ = s.DropTables(ctx) })
		return s
	}
	storagetest.RunDBConformance(t, mk("agno_test_db"))
	storagetest.RunMetricsConformance(t, mk("agno_test_metrics"))
	s, _ := New(ctx, Config{Database: "agno_test_search"})
	_ = s.DropTables(ctx)
	defer s.DropTables(ctx)
	storagetest.RunSessionSearchConformance(t, s)
}

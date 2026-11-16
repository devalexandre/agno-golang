// Valkey storage backend (Redis-protocol compatible).
//
// Start Valkey: docker run -p 6379:6379 valkey/valkey:8
// Run:          VALKEY_URL=valkey://localhost:6379/0 go run ./cookbook/storage/valkey
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/storage/valkey"
)

func main() {
	ctx := context.Background()
	db, err := valkey.New(valkey.Config{Prefix: "agno_cookbook"})
	if err != nil {
		log.Fatal(err)
	}
	demo(ctx, db)
}

// demo writes a session and a run, reads them back, searches sessions with a
// filter tree and calculates daily metrics.
func demo(ctx context.Context, db store) {
	if err := db.CreateTables(ctx); err != nil {
		log.Fatalf("create tables: %v", err)
	}
	sess := storage.NewAgentSession("cookbook-session", "alice", "support-agent")
	sess.SessionData["session_name"] = "Billing question"
	if err := db.UpdateSession(ctx, sess); err != nil {
		log.Fatalf("save session: %v", err)
	}
	run := &storage.AgentRun{
		ID: "cookbook-run", SessionID: sess.SessionID, UserID: "alice",
		UserMessage: "Why was I charged twice?", AgentMessage: "Let me check that for you.",
		RunData:   map[string]interface{}{},
		Metrics:   map[string]interface{}{"input_tokens": 42, "output_tokens": 12, "model": "gpt-4o-mini", "model_provider": "openai"},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.UpdateRun(ctx, run); err != nil {
		log.Fatalf("save run: %v", err)
	}

	got, err := db.ReadSession(ctx, sess.SessionID)
	if err != nil || got == nil {
		log.Fatalf("read session: %v", err)
	}
	fmt.Printf("session %s of %s (agent %s)\n", got.SessionID, got.UserID, got.AgentID)

	filter := storage.And(
		storage.Where("agent_id", storage.FilterEq, "support-agent"),
		storage.Where("session_name", storage.FilterContains, "billing"),
	)
	found, total, err := db.SearchSessions(ctx, filter, storage.SessionSearchOptions{Limit: 10})
	if err != nil {
		log.Fatalf("search: %v", err)
	}
	fmt.Printf("search matched %d session(s), first page has %d\n", total, len(found))

	if _, err := db.CalculateMetrics(ctx, nil); err != nil {
		log.Fatalf("metrics: %v", err)
	}
	today := time.Now().UTC()
	rows, _, err := db.GetMetricsByDate(ctx, &today, &today)
	if err != nil {
		log.Fatalf("get metrics: %v", err)
	}
	for _, r := range rows {
		fmt.Printf("%s: %d agent run(s), %d input tokens, %d user(s)\n", r.Date, r.AgentRunsCount, r.TokenMetrics["input_tokens"], r.UsersCount)
	}
}

// store is every contract the demo uses.
type store interface {
	storage.DB
	storage.SessionSearcher
	storage.MetricsStore
}

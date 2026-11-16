// Session filters: search sessions with an AND/OR/NOT filter tree (port of
// Python agno/db/session_filters.py).
//
// Filters can be built in Go (storage.And/Or/Not/Where/WhereIn) or parsed from
// the same JSON the AgentOS API accepts. Fields: session_id, session_name,
// user_id, agent_id, team_id, workflow_id (EQ, NEQ, IN, CONTAINS, STARTSWITH),
// session_type (EQ, NEQ, IN), created_at/updated_at (EQ, NEQ, GT, GTE, LT, LTE;
// epoch seconds or ISO-8601). Max depth 10, max 100 conditions/values per node.
// A missing field never matches (SQL NULL semantics), even under NOT.
//
// Runs fully in-process (in-memory store + SQLite through the shared SQL core):
//
//	go run ./cookbook/storage/session_filters
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"

	"github.com/devalexandre/agno-golang/agno/storage"
	inmemory "github.com/devalexandre/agno-golang/agno/storage/in_memory"
	"github.com/devalexandre/agno-golang/agno/storage/sqlstore"
)

type searchable interface {
	storage.DB
	storage.SessionSearcher
}

func seed(ctx context.Context, db searchable) {
	if err := db.CreateTables(ctx); err != nil {
		log.Fatal(err)
	}
	for i, s := range []struct{ id, user, agent, name string }{
		{"s1", "alice", "support", "Billing question"},
		{"s2", "alice", "sales", "Pricing request"},
		{"s3", "bob", "support", "Login problem"},
		{"s4", "", "support", "billing follow-up"},
	} {
		sess := storage.NewAgentSession(s.id, s.user, s.agent)
		sess.CreatedAt = int64(1_700_000_000 + i*3600)
		sess.SessionData["session_name"] = s.name
		if err := db.CreateSession(ctx, sess); err != nil {
			log.Fatal(err)
		}
	}
}

func show(ctx context.Context, label string, db searchable, f *storage.SessionFilter) {
	sessions, total, err := db.SearchSessions(ctx, f, storage.SessionSearchOptions{SortOrder: "asc"})
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.SessionID
	}
	fmt.Printf("  %-40s -> %v (total %d)\n", label, ids, total)
}

func main() {
	ctx := context.Background()

	// A filter in the AgentOS JSON format.
	parsed, err := storage.ParseSessionFilter([]byte(`{
		"op": "AND",
		"conditions": [
			{"op": "EQ", "key": "agent_id", "value": "support"},
			{"op": "OR", "conditions": [
				{"op": "CONTAINS", "key": "session_name", "value": "billing"},
				{"op": "IN", "key": "user_id", "values": ["bob"]}
			]}
		]
	}`))
	if err != nil {
		log.Fatal(err)
	}

	filters := []struct {
		label string
		f     *storage.SessionFilter
	}{
		{"JSON: support AND (billing OR bob)", parsed},
		{"user_id NEQ alice (NULL never matches)", storage.Where("user_id", storage.FilterNeq, "alice")},
		{"NOT agent_id EQ support", storage.Not(storage.Where("agent_id", storage.FilterEq, "support"))},
		{"created_at >= 2023-11-14T23:13:20Z", storage.Where("created_at", storage.FilterGte, "2023-11-14T23:13:20Z")},
		{"session_id STARTSWITH s", storage.Where("session_id", storage.FilterStartsWith, "s")},
	}

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)

	backends := []struct {
		name string
		db   searchable
	}{
		{"in-memory", inmemory.New()},
		{"sql (sqlite via sqlstore; native columns are filtered in SQL)", sqlstore.New(sqlDB, sqlstore.Dialect{Driver: "sqlite3"}, "agno")},
	}
	for _, b := range backends {
		seed(ctx, b.db)
		fmt.Println(b.name + ":")
		for _, f := range filters {
			show(ctx, f.label, b.db, f.f)
		}
	}

	// Validation errors are reported before any query runs.
	_, err = storage.NormalizeSessionFilter(storage.Where("created_at", storage.FilterContains, "x"))
	fmt.Println("invalid filter:", err)

	// SQL translation used by the SQL backends.
	clause, args, _ := storage.SessionFilterToSQL(storage.And(
		storage.Where("agent_id", storage.FilterEq, "support"),
		storage.Where("created_at", storage.FilterLt, 1_700_003_600),
	), storage.SessionFilterSQLOptions{Columns: map[string]string{"agent_id": "agent_id", "created_at": "created_at"}})
	fmt.Printf("SQL: %s %v\n", clause, args)
}

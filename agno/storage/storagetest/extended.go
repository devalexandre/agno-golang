package storagetest

import (
	"context"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
)

// SearchableDB is a storage.DB with session search.
type SearchableDB interface {
	storage.DB
	storage.SessionSearcher
}

// MetricsDB is a storage.DB with the metrics API.
type MetricsDB interface {
	storage.DB
	storage.MetricsStore
}

// RunSessionSearchConformance exercises SearchSessions with filter trees,
// sorting and pagination. db must be empty.
func RunSessionSearchConformance(t *testing.T, db SearchableDB) {
	t.Helper()
	ctx := context.Background()
	if err := db.CreateTables(ctx); err != nil {
		t.Fatalf("CreateTables: %v", err)
	}
	mk := func(id, user, agent, name string, created int64) {
		s := storage.NewAgentSession(id, user, agent)
		s.CreatedAt, s.UpdatedAt = created, created+10
		if name != "" {
			s.SessionData["session_name"] = name
		}
		if err := db.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession(%s): %v", id, err)
		}
	}
	mk("f1", "alice", "support", "Billing question", 100)
	mk("f2", "alice", "sales", "Pricing 50% off", 200)
	mk("f3", "bob", "support", "", 300)
	mk("f4", "", "support", "billing follow-up", 400)

	search := func(f *storage.SessionFilter, opts storage.SessionSearchOptions) ([]string, int) {
		t.Helper()
		got, total, err := db.SearchSessions(ctx, f, opts)
		if err != nil {
			t.Fatalf("SearchSessions(%+v): %v", f, err)
		}
		ids := make([]string, len(got))
		for i, s := range got {
			ids[i] = s.SessionID
		}
		return ids, total
	}
	expect := func(name string, got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %v want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v want %v", name, got, want)
			}
		}
	}
	asc := storage.SessionSearchOptions{SortOrder: "asc"}

	ids, total := search(nil, storage.SessionSearchOptions{})
	expect("no filter desc", ids, "f4", "f3", "f2", "f1")
	if total != 4 {
		t.Fatalf("total = %d", total)
	}

	ids, _ = search(storage.Where("agent_id", storage.FilterEq, "support"), asc)
	expect("EQ", ids, "f1", "f3", "f4")

	ids, _ = search(storage.Where("session_name", storage.FilterContains, "BILLING"), asc)
	expect("CONTAINS case-insensitive", ids, "f1", "f4")

	ids, _ = search(storage.Where("session_name", storage.FilterContains, "50%"), asc)
	expect("CONTAINS escapes wildcards", ids, "f2")

	ids, _ = search(storage.Where("session_id", storage.FilterStartsWith, "f"), asc)
	expect("STARTSWITH", ids, "f1", "f2", "f3", "f4")

	ids, _ = search(storage.And(
		storage.Where("agent_id", storage.FilterEq, "support"),
		storage.Where("created_at", storage.FilterGte, 200),
	), asc)
	expect("AND + datetime", ids, "f3", "f4")

	ids, _ = search(storage.Or(
		storage.WhereIn("user_id", "bob"),
		storage.Where("agent_id", storage.FilterEq, "sales"),
	), asc)
	expect("OR + IN", ids, "f2", "f3")

	// NULL semantics: f4 has no user, so it matches neither NEQ nor NOT EQ.
	ids, _ = search(storage.Where("user_id", storage.FilterNeq, "alice"), asc)
	expect("NEQ null semantics", ids, "f3")
	ids, _ = search(storage.Not(storage.Where("user_id", storage.FilterEq, "alice")), asc)
	expect("NOT null semantics", ids, "f3")

	ids, _ = search(storage.Where("session_type", storage.FilterEq, "agent"), asc)
	expect("session_type", ids, "f1", "f2", "f3", "f4")

	ids, _ = search(storage.Where("created_at", storage.FilterLt, "1970-01-01T00:05:01Z"), asc)
	expect("datetime string", ids, "f1", "f2", "f3")

	alice := "alice"
	ids, total = search(nil, storage.SessionSearchOptions{UserID: &alice, Limit: 1, Page: 2, SortOrder: "asc"})
	expect("user + pagination", ids, "f2")
	if total != 2 {
		t.Fatalf("paginated total = %d", total)
	}

	ids, _ = search(nil, storage.SessionSearchOptions{SortBy: "updated_at", SortOrder: "desc", Limit: 2})
	expect("sort updated_at", ids, "f4", "f3")

	if _, _, err := db.SearchSessions(ctx, storage.Where("nope", storage.FilterEq, "x"), storage.SessionSearchOptions{}); err == nil {
		t.Fatal("expected error for unknown field")
	}
	if _, _, err := db.SearchSessions(ctx, storage.Where("created_at", storage.FilterContains, "x"), storage.SessionSearchOptions{}); err == nil {
		t.Fatal("expected error for unsupported operator")
	}
}

// RunMetricsConformance exercises CalculateMetrics and the metrics readers.
// db must be empty.
func RunMetricsConformance(t *testing.T, db MetricsDB) {
	t.Helper()
	ctx := context.Background()
	if err := db.CreateTables(ctx); err != nil {
		t.Fatalf("CreateTables: %v", err)
	}
	day1 := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)

	mkSession := func(id, user, agent string, at time.Time) {
		s := storage.NewAgentSession(id, user, agent)
		s.CreatedAt, s.UpdatedAt = at.Unix(), at.Unix()
		if err := db.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	mkRun := func(id, session, user string, at time.Time, in, out int, model string) {
		r := &storage.AgentRun{
			ID: id, SessionID: session, UserID: user, CreatedAt: at, UpdatedAt: at,
			RunData: map[string]interface{}{},
			Metrics: map[string]interface{}{"input_tokens": float64(in), "output_tokens": float64(out), "total_tokens": float64(in + out), "cost": 0.5, "model": model, "model_provider": "openai"},
		}
		if err := db.CreateRun(ctx, r); err != nil {
			t.Fatalf("CreateRun: %v", err)
		}
	}
	mkSession("m1", "alice", "agent-a", day1)
	mkSession("m2", "bob", "agent-a", day1)
	mkSession("m3", "", "agent-b", day2)
	mkRun("mr1", "m1", "alice", day1, 10, 5, "gpt-4o")
	mkRun("mr2", "m1", "alice", day1, 20, 5, "gpt-4o")
	mkRun("mr3", "m2", "bob", day1, 1, 1, "gpt-4o-mini")
	team := &storage.AgentRun{ID: "mr4", SessionID: "m3", CreatedAt: day2, UpdatedAt: day2,
		RunData: map[string]interface{}{"team_id": "team-x"}, Metrics: map[string]interface{}{"input_tokens": float64(7)}}
	if err := db.CreateRun(ctx, team); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	calculated, err := db.CalculateMetrics(ctx, nil)
	if err != nil {
		t.Fatalf("CalculateMetrics: %v", err)
	}
	if len(calculated) == 0 {
		t.Fatal("CalculateMetrics returned no rows")
	}

	from, to := day1, day2
	rows, _, err := db.GetMetrics(ctx, &from, &to, nil)
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	// day1: alice, bob ; day2: unowned
	if len(rows) != 3 {
		t.Fatalf("GetMetrics rows = %d: %+v", len(rows), rows)
	}
	alice := "alice"
	aliceRows, _, err := db.GetMetrics(ctx, &from, &to, &alice)
	if err != nil || len(aliceRows) != 1 {
		t.Fatalf("GetMetrics(alice) = %v, %v", aliceRows, err)
	}
	a := aliceRows[0]
	if a.AgentSessionsCount != 1 || a.AgentRunsCount != 2 || a.TokenMetrics["input_tokens"] != 30 || a.UsersCount != 1 {
		t.Fatalf("alice metrics wrong: %+v", a)
	}
	if a.Cost == nil || *a.Cost != 1.0 {
		t.Fatalf("alice cost wrong: %v", a.Cost)
	}
	if len(a.ModelMetrics) != 1 || a.ModelMetrics[0].Count != 2 {
		t.Fatalf("alice model metrics wrong: %+v", a.ModelMetrics)
	}
	unowned := ""
	un, _, _ := db.GetMetrics(ctx, &from, &to, &unowned)
	if len(un) != 1 || un[0].UserID != nil || un[0].TeamRunsCount != 1 || un[0].AgentSessionsCount != 1 {
		t.Fatalf("unowned metrics wrong: %+v", un)
	}

	byDate, _, err := db.GetMetricsByDate(ctx, &from, &to)
	if err != nil || len(byDate) != 2 {
		t.Fatalf("GetMetricsByDate = %d rows, %v", len(byDate), err)
	}
	if byDate[0].UsersCount != 2 || byDate[0].AgentRunsCount != 3 || byDate[0].TokenMetrics["input_tokens"] != 31 {
		t.Fatalf("day1 aggregate wrong: %+v", byDate[0])
	}

	totals, _, err := db.GetMetricsTotals(ctx, from, to, nil)
	if err != nil {
		t.Fatalf("GetMetricsTotals: %v", err)
	}
	if totals.UsersCount != 2 || totals.AgentRunsCount != 3 || totals.TeamRunsCount != 1 || totals.TokenMetrics["input_tokens"] != 38 {
		t.Fatalf("totals wrong: %+v", totals)
	}

	comp, _, err := db.GetComponentMetrics(ctx, from, from, nil)
	if err != nil || len(comp) != 2 {
		t.Fatalf("GetComponentMetrics(day1) = %d, %v", len(comp), err)
	}

	st, err := db.GetMetricsRefreshStatus(ctx)
	if err != nil || st == nil || st.Status != "completed" {
		t.Fatalf("GetMetricsRefreshStatus = %+v, %v", st, err)
	}

	// Deleting a run and rebuilding updates the day.
	if err := db.DeleteRun(ctx, "mr2"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CalculateMetrics(ctx, &from); err != nil {
		t.Fatalf("CalculateMetrics(rebuild): %v", err)
	}
	aliceRows, _, _ = db.GetMetrics(ctx, &from, &to, &alice)
	if len(aliceRows) != 1 || aliceRows[0].AgentRunsCount != 1 {
		t.Fatalf("rebuild did not apply: %+v", aliceRows)
	}
}

// RunKnowledgeConformance exercises storage.KnowledgeStorage.
func RunKnowledgeConformance(t *testing.T, ks storage.KnowledgeStorage) {
	t.Helper()
	for i, name := range []string{"b-doc", "a-doc", "c-doc"} {
		row := storage.NewKnowledgeRow(name, name, "desc")
		ts := int64(100 + i)
		row.CreatedAt = &ts
		row.Metadata["n"] = float64(i)
		if _, err := ks.UpsertKnowledgeContent(row); err != nil {
			t.Fatalf("UpsertKnowledgeContent: %v", err)
		}
	}
	got, err := ks.GetKnowledgeContent("a-doc")
	if err != nil || got == nil || got.Name != "a-doc" || got.Metadata["n"] != float64(1) {
		t.Fatalf("GetKnowledgeContent = %+v, %v", got, err)
	}
	if miss, err := ks.GetKnowledgeContent("missing"); err != nil || miss != nil {
		t.Fatalf("GetKnowledgeContent(missing) = %v, %v", miss, err)
	}
	limit, page, by, order := 2, 1, "name", "asc"
	rows, total, err := ks.GetKnowledgeContents(&limit, &page, &by, &order)
	if err != nil || total != 3 || len(rows) != 2 || rows[0].ID != "a-doc" || rows[1].ID != "b-doc" {
		t.Fatalf("GetKnowledgeContents = %v total=%d err=%v", rows, total, err)
	}
	if err := ks.DeleteKnowledgeContent("a-doc"); err != nil {
		t.Fatal(err)
	}
	if gone, _ := ks.GetKnowledgeContent("a-doc"); gone != nil {
		t.Fatal("DeleteKnowledgeContent did not remove the row")
	}
}

// RunStorageConformance exercises the mode-based storage.Storage contract in
// team mode (the contract consumed by teams).
func RunStorageConformance(t *testing.T, st storage.Storage) {
	t.Helper()
	if err := st.Create(); err != nil {
		t.Fatalf("Create: %v", err)
	}
	st.SetMode(storage.TeamMode)
	defer st.SetMode(storage.AgentMode)
	for i, id := range []string{"t1", "t2"} {
		ts := storage.NewTeamSession(id, "u1", "team-a")
		ts.CreatedAt = int64(100 + i)
		ts.TeamData["k"] = id
		if _, err := st.Upsert(ts); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}
	got, err := st.Read("t1", nil)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	ts, ok := got.(*storage.TeamSession)
	if !ok || ts.TeamID != "team-a" || ts.TeamData["k"] != "t1" {
		t.Fatalf("Read = %#v", got)
	}
	other := "u2"
	if got, _ := st.Read("t1", &other); got != nil {
		t.Fatalf("Read with other user should be nil, got %#v", got)
	}
	team := "team-a"
	limit := 1
	recent, err := st.GetRecentSessions(nil, &team, &limit)
	if err != nil || len(recent) != 1 || recent[0].(*storage.TeamSession).SessionID != "t2" {
		t.Fatalf("GetRecentSessions = %v, %v", recent, err)
	}
	if err := st.DeleteSession(context.Background(), "t1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if got, _ := st.Read("t1", nil); got != nil {
		t.Fatal("DeleteSession did not remove the team session")
	}
}

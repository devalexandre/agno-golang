// Metrics: calculate and read daily usage metrics from stored sessions and
// runs (port of Python BaseDb.calculate_metrics / get_metrics /
// get_metrics_by_date / get_component_metrics / get_metrics_totals).
//
// CalculateMetrics rebuilds every day that changed since the last
// calculation (or every day since rebuildFrom) into "component metrics" rows:
// one per day, user and agent/team/workflow, with session and run counts,
// token counts, cost and model usage read from AgentRun.Metrics. The readers
// aggregate those rows per owner, per day or over a range.
//
// The same API exists on the in-memory store, the SQL stores (sqlite via
// sqlstore, postgres, mysql, singlestore) and every kvstore backend (redis,
// valkey, mongo, dynamo, firestore, gcs_json, surrealdb, clickhouse).
//
// Runs fully in-process:
//
//	go run ./cookbook/storage/metrics
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/storage"
	inmemory "github.com/devalexandre/agno-golang/agno/storage/in_memory"
)

func main() {
	ctx := context.Background()
	db := inmemory.New()

	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	today := time.Now().UTC()

	addSession := func(id, user, agent string, at time.Time) {
		s := storage.NewAgentSession(id, user, agent)
		s.CreatedAt, s.UpdatedAt = at.Unix(), at.Unix()
		if err := db.CreateSession(ctx, s); err != nil {
			log.Fatal(err)
		}
	}
	addRun := func(id, session, user string, at time.Time, in, out int, model string, cost float64) {
		r := &storage.AgentRun{
			ID: id, SessionID: session, UserID: user, CreatedAt: at, UpdatedAt: at,
			RunData: map[string]interface{}{},
			Metrics: map[string]interface{}{
				"input_tokens": in, "output_tokens": out, "total_tokens": in + out,
				"cost": cost, "model": model, "model_provider": "openai",
			},
		}
		if err := db.CreateRun(ctx, r); err != nil {
			log.Fatal(err)
		}
	}

	addSession("s1", "alice", "support-agent", yesterday)
	addRun("r1", "s1", "alice", yesterday, 120, 40, "gpt-4o", 0.002)
	addRun("r2", "s1", "alice", yesterday, 80, 30, "gpt-4o", 0.001)
	addSession("s2", "bob", "support-agent", today)
	addRun("r3", "s2", "bob", today, 60, 20, "gpt-4o-mini", 0.0002)
	// A team run: RunData names the component.
	addSession("s3", "", "", today)
	if err := db.CreateRun(ctx, &storage.AgentRun{
		ID: "r4", SessionID: "s3", CreatedAt: today, UpdatedAt: today,
		RunData: map[string]interface{}{"team_id": "research-team"},
		Metrics: map[string]interface{}{"input_tokens": 300, "output_tokens": 90},
	}); err != nil {
		log.Fatal(err)
	}

	if _, err := db.CalculateMetrics(ctx, nil); err != nil {
		log.Fatal(err)
	}
	status, _ := db.GetMetricsRefreshStatus(ctx)
	fmt.Printf("refresh status: %s\n\n", status.Status)

	byDate, _, err := db.GetMetricsByDate(ctx, &yesterday, &today)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("per day (everyone):")
	for _, r := range byDate {
		fmt.Printf("  %s users=%d agent_runs=%d team_runs=%d input_tokens=%d\n",
			r.Date, r.UsersCount, r.AgentRunsCount, r.TeamRunsCount, r.TokenMetrics["input_tokens"])
	}

	alice := "alice"
	perUser, _, _ := db.GetMetrics(ctx, &yesterday, &today, &alice)
	fmt.Println("\nalice:")
	for _, r := range perUser {
		raw, _ := json.Marshal(r.ToDict()["token_metrics"])
		for _, m := range r.ModelMetrics {
			fmt.Printf("  %s model %s/%s used in %d run(s)\n", r.Date, m.ModelProvider, m.ModelID, m.Count)
		}
		fmt.Printf("  %s runs=%d token_metrics=%s\n", r.Date, r.AgentRunsCount, raw)
	}

	components, _, _ := db.GetComponentMetrics(ctx, yesterday, today, nil)
	fmt.Println("\ncomponents:")
	for _, c := range components {
		fmt.Printf("  %s %-8s %-15s user=%-6q sessions=%d runs=%d\n", c.Date, c.ComponentType, c.ComponentID, c.UserID, c.SessionsCount, c.RunsCount)
	}

	totals, _, _ := db.GetMetricsTotals(ctx, yesterday, today, nil)
	fmt.Printf("\ntotals: users=%d agent_runs=%d team_runs=%d total_tokens=%d cost=%.4f\n",
		totals.UsersCount, totals.AgentRunsCount, totals.TeamRunsCount, totals.TokenMetrics["total_tokens"], *totals.Cost)
}

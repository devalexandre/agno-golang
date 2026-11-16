// Metrics routes (/metrics, /metrics/refresh, /metrics/refresh/status).
//
// Shows: daily aggregated metrics (runs and sessions per component type,
// users, token totals) computed from the sessions stored in the AgentOS
// database. POST /metrics/refresh recalculates them; GET /metrics reads them
// for an optional date window. agnoos.CalculateMetrics does the same from Go.
//
// Needs: nothing external; the example seeds sessions and calls no model.
//
//	go run ./cookbook/os/metrics
//	curl -X POST localhost:7777/metrics/refresh
//	curl "localhost:7777/metrics?starting_date=2020-01-01&ending_date=2030-12-31"
//	curl localhost:7777/metrics/refresh/status
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	db := agnoos.NewInMemoryDB("metrics-db")
	now := time.Now().Unix()
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("s%d", i)
		err := db.UpsertSession(context.Background(), &agnoos.SessionRecord{
			SessionID: id, SessionType: agnoos.SessionTypeAgent, AgentID: "support",
			UserID: fmt.Sprintf("user%d", i%2), CreatedAt: now, UpdatedAt: now,
			Runs: []*agnoos.RunRecord{{
				RunID: "r" + id, SessionID: id, AgentID: "support", Status: "COMPLETED", CreatedAt: now,
				Metrics: map[string]interface{}{"input_tokens": 10, "output_tokens": 20, "total_tokens": 30},
			}},
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	days, err := agnoos.CalculateMetrics(context.Background(), db)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("calculated %d day(s), %d agent runs today\n", len(days), days[len(days)-1].AgentRunsCount)

	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:     "metrics-os",
		DB:       db,
		Settings: &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}

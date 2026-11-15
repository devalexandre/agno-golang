// Scheduler HTTP routes: the Python /schedules API on a gin router, backed
// by a durable SQLite store and a running poller.
//
// The executor is HTTPExecutor, which (like Python's ScheduleExecutor) calls
// the AgentOS endpoint stored on each schedule; point AGENTOS_URL at a
// running AgentOS (default http://localhost:7777). Schedules whose endpoint
// is unreachable are recorded as failed runs.
//
// Env: PORT (default 8090), SCHEDULER_DB (default ./scheduler.db),
// AGENTOS_URL, AGENTOS_TOKEN (optional bearer token).
//
// Try:
//
//	curl -XPOST localhost:8090/schedules -d '{"name":"nightly","cron_expr":"0 3 * * *","endpoint":"/agents/my-agent/runs","payload":{"message":"hi"}}'
//	curl localhost:8090/schedules
//	curl -XPOST localhost:8090/schedules/<id>/trigger
//	curl localhost:8090/schedules/<id>/runs
//
// In an AgentOS app, mount with scheduler.RegisterRoutes(router.Group("/"), manager, opts).
package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/scheduler"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	ctx := context.Background()
	db, err := sql.Open("sqlite", env("SCHEDULER_DB", "scheduler.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	store, err := scheduler.NewSQLStore(db, scheduler.SQLite, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}

	exec := &scheduler.HTTPExecutor{BaseURL: env("AGENTOS_URL", "http://localhost:7777"), Token: os.Getenv("AGENTOS_TOKEN")}
	sched := scheduler.New(store, exec)
	sched.PollInterval = 15 * time.Second
	sched.Start(ctx)
	defer sched.Stop(ctx) //nolint:errcheck

	r := gin.Default()
	scheduler.RegisterRoutes(r.Group("/"), scheduler.NewManager(store), scheduler.RouterOptions{Scheduler: sched})
	log.Fatal(r.Run(":" + env("PORT", "8090")))
}

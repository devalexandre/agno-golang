// Durable job queue worker: background runs survive restarts.
//
// Jobs are committed to a SQLite table before they run, a worker claims and
// heartbeats them, transient errors are retried with backoff, permanent ones
// fail at once, and a cancelled job is tombstoned. Run it twice: jobs left
// queued by a killed process are picked up by the next one.
//
// For Postgres (multi-replica, FOR UPDATE SKIP LOCKED), open the DB with
// lib/pq and use jobqueue.NewSQLStore(db, jobqueue.Postgres, "").
//
// Env: JOBQUEUE_DB (default ./jobqueue.db). Offline; no model needed.
// Run: go run ./cookbook/jobqueue/worker
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/devalexandre/agno-golang/agno/jobqueue"
	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()
	path := os.Getenv("JOBQUEUE_DB")
	if path == "" {
		path = "jobqueue.db"
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	store, err := jobqueue.NewSQLStore(db, jobqueue.SQLite, "")
	if err != nil {
		log.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}

	var flaky int32
	handler := func(ctx context.Context, job *jobqueue.Job) (jobqueue.Outcome, error) {
		switch job.Payload["kind"] {
		case "flaky":
			if atomic.AddInt32(&flaky, 1) == 1 {
				return jobqueue.Outcome{}, errors.New("upstream timeout") // retried
			}
		case "invalid":
			return jobqueue.Outcome{}, jobqueue.Permanent(errors.New("bad input")) // never retried
		case "slow":
			select {
			case <-time.After(10 * time.Second):
			case <-ctx.Done(): // cancellation reaches the handler through ctx
				return jobqueue.Outcome{}, ctx.Err()
			}
		}
		fmt.Printf("ran job %s (%s) attempt %d\n", job.ID[:8], job.Payload["kind"], job.Attempt)
		return jobqueue.Outcome{}, nil
	}

	w := jobqueue.NewWorker(store, handler, jobqueue.Config{
		MaxConcurrency: 4, MaxAttempts: 3, RetryDelay: time.Second, PollInterval: 200 * time.Millisecond,
	})
	w.Start(ctx)

	var ids []string
	for _, kind := range []string{"ok", "flaky", "invalid", "slow"} {
		res, err := w.Enqueue(ctx, jobqueue.EnqueueOptions{ComponentType: "agent", ComponentID: "assistant",
			SessionID: "s-" + kind, Payload: map[string]any{"kind": kind}, IdempotencyKey: "demo-" + kind + "-" + time.Now().Format("150405")})
		if err != nil {
			log.Fatal(err)
		}
		ids = append(ids, res.Job.ID)
	}
	time.Sleep(500 * time.Millisecond)
	_, _ = w.Cancel(ctx, ids[3]) // cancel the slow job while it runs
	time.Sleep(3 * time.Second)
	_ = w.Stop(ctx)

	for _, id := range ids {
		j, _ := store.Get(ctx, id)
		errMsg := ""
		if j.Error != nil {
			errMsg = *j.Error
		}
		fmt.Printf("%s kind=%-8v status=%-9s attempts=%d error=%q\n", id[:8], j.Payload["kind"], j.Status, j.Attempt, errMsg)
	}
	stats, _ := store.Stats(ctx)
	fmt.Printf("queue stats: %v\n", stats.Counts)
}

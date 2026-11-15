// Per-session ordering: at most one live run per session, in submission order.
//
// Three turns of one conversation are enqueued with SerializeSession; even
// with spare worker capacity they run strictly one after another, while a
// different session runs in parallel. A paused turn (awaiting human input)
// keeps holding its session until it is continued.
//
// Requirements: none (in-memory store). Run: go run ./cookbook/jobqueue/per_session
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/jobqueue"
)

func main() {
	ctx := context.Background()
	store := jobqueue.NewMemoryStore()
	start := time.Now()
	var mu sync.Mutex

	handler := func(ctx context.Context, job *jobqueue.Job) (jobqueue.Outcome, error) {
		turn := job.Payload["turn"].(string)
		mu.Lock()
		fmt.Printf("%5dms start  %s/%s\n", time.Since(start).Milliseconds(), job.SessionID, turn)
		mu.Unlock()
		time.Sleep(150 * time.Millisecond)
		if turn == "approve-transfer" && job.Payload["continue"] == nil {
			fmt.Printf("%5dms paused %s/%s (waiting for approval)\n", time.Since(start).Milliseconds(), job.SessionID, turn)
			return jobqueue.Outcome{Status: jobqueue.StatusPaused}, nil
		}
		return jobqueue.Outcome{}, nil
	}
	w := jobqueue.NewWorker(store, handler, jobqueue.Config{MaxConcurrency: 8, PollInterval: 20 * time.Millisecond})
	w.Start(ctx)
	defer w.Stop(ctx) //nolint:errcheck

	enqueue := func(session, turn string) string {
		res, err := w.Enqueue(ctx, jobqueue.EnqueueOptions{SessionID: session, SerializeSession: true,
			Payload: map[string]any{"turn": turn}})
		if err != nil || !res.Accepted {
			log.Fatalf("enqueue: %v %+v", err, res)
		}
		return res.Job.ID
	}
	enqueue("alice", "hello")
	pausedID := enqueue("alice", "approve-transfer")
	last := enqueue("alice", "thanks")
	enqueue("bob", "hi")
	enqueue("bob", "bye")

	time.Sleep(800 * time.Millisecond)
	blocker, _ := store.SessionBlocker(ctx, last)
	fmt.Printf("alice/thanks waits behind %s (the paused turn: %v)\n", blocker[:8], blocker == pausedID)

	outcome, _, _ := w.Continue(ctx, pausedID, map[string]any{"approved": true})
	fmt.Printf("continue -> %s\n", outcome)
	time.Sleep(800 * time.Millisecond)

	stats, _ := store.Stats(ctx)
	fmt.Printf("final: %v\n", stats.Counts)
}

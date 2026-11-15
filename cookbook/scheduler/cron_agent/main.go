// Cron agent: schedule an agent run with the in-process scheduler.
//
// Shows cron, interval and one-shot schedules, the poller executing due
// schedules against a registered "agent" (a stub runner, so this runs
// offline), retries, manual trigger and run history.
//
// To schedule a real agent, wrap it in a RunnerFunc:
//
//	scheduler.RunnerFunc(func(ctx context.Context, r scheduler.RunRequest) (*scheduler.RunOutput, error) {
//	    resp, err := myAgent.Run(r.Input)
//	    if err != nil { return nil, err }
//	    return &scheduler.RunOutput{Content: resp.TextContent}, nil
//	})
//
// Requirements: none. Run: go run ./cookbook/scheduler/cron_agent
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/devalexandre/agno-golang/agno/scheduler"
)

func main() {
	ctx := context.Background()
	store := scheduler.NewMemoryStore()
	manager := scheduler.NewManager(store)

	var calls int32
	reporter := scheduler.RunnerFunc(func(ctx context.Context, r scheduler.RunRequest) (*scheduler.RunOutput, error) {
		// Fail the very first call to show retries.
		if atomic.AddInt32(&calls, 1) == 1 {
			return nil, errors.New("model temporarily unavailable")
		}
		return &scheduler.RunOutput{RunID: fmt.Sprintf("run-%d", calls), Content: "REPORT: " + strings.ToUpper(r.Input)}, nil
	})
	exec := scheduler.NewComponentExecutor().RegisterAgent("reporter", reporter)

	sched := scheduler.New(store, exec)
	sched.PollInterval = 200 * time.Millisecond
	sched.Sleep = func(context.Context, time.Duration) error { return nil } // skip retry delays in the demo

	// 1) Cron: weekdays at 09:00 São Paulo time (only computed here, not waited for).
	cron, err := manager.Create(ctx, scheduler.CreateRequest{
		Name: "daily-report", CronExpr: "0 9 * * MON-FRI", Timezone: "America/Sao_Paulo",
		TargetType: "agent", TargetID: "reporter", Payload: map[string]any{"message": "daily metrics"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("cron schedule %q next run: %s\n", cron.Name, time.Unix(*cron.NextRunAt, 0).Format(time.RFC1123))

	// 2) One-shot, due now, with up to 2 retries.
	retries := 2
	runAt := time.Now().Unix()
	once, err := manager.Create(ctx, scheduler.CreateRequest{
		Name: "kickoff", RunAt: &runAt, TargetType: "agent", TargetID: "reporter",
		MaxRetries: &retries, Payload: map[string]any{"message": "kickoff summary"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 3) Interval: every hour.
	if _, err := manager.Create(ctx, scheduler.CreateRequest{Name: "hourly-sync", IntervalSeconds: 3600,
		Endpoint: "/agents/reporter/runs", Payload: map[string]any{"message": "sync"}}); err != nil {
		log.Fatal(err)
	}

	sched.Start(ctx)
	time.Sleep(time.Second)
	sched.Wait()
	_ = sched.Stop(ctx)

	runs, _, _ := manager.GetRuns(ctx, once.ID, nil, 10, 1)
	fmt.Printf("one-shot %q attempts (newest first):\n", once.Name)
	for _, r := range runs {
		fmt.Printf("  attempt=%d status=%s error=%v output=%v\n", r.Attempt, r.Status, deref(r.Error), r.Output)
	}
	after, _ := manager.Get(ctx, once.ID, nil)
	fmt.Printf("one-shot enabled after firing: %v\n", after.Enabled)

	// Manual trigger of the cron schedule (does not move its next_run_at).
	run, err := sched.Trigger(ctx, cron.ID, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("manual trigger: status=%s output=%v\n", run.Status, run.Output)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

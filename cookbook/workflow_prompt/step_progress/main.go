// StepProgress: a function step reports progress while it is still running.
//
// (*StepInput).ReportProgress(content, data) emits a StepProgress event on the
// workflow event stream; it never replaces the step output. The event's
// Metadata carries step_name, step_index and the 1-based retry attempt, and
// progress from steps nested in Parallel/Condition/Loop/Steps is attributed
// to the right step. Outside a running workflow the call is a no-op, so
// executors can call it unconditionally.
//
// Runs offline (no model needed):
//
//	go run ./cookbook/workflow_prompt/step_progress
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	batches := []string{"users", "orders", "invoices"}

	ingest, err := v2.NewStep(v2.WithName("ingest"), v2.WithExecutor(func(in *v2.StepInput) (*v2.StepOutput, error) {
		for i, b := range batches {
			time.Sleep(50 * time.Millisecond) // pretend to work
			in.ReportProgress(fmt.Sprintf("batch %d/%d: %s", i+1, len(batches), b),
				map[string]interface{}{"done": i + 1, "total": len(batches)})
		}
		return &v2.StepOutput{Content: fmt.Sprintf("ingested %d batches", len(batches))}, nil
	}))
	if err != nil {
		log.Fatal(err)
	}

	// A flaky step: progress events carry the retry attempt.
	attempts := 0
	publish, err := v2.NewStep(v2.WithName("publish"), v2.WithMaxRetries(1), v2.WithExecutor(func(in *v2.StepInput) (*v2.StepOutput, error) {
		attempts++
		in.ReportProgress("uploading report", nil)
		if attempts == 1 {
			return nil, errors.New("temporary network error")
		}
		return &v2.StepOutput{Content: "published"}, nil
	}))
	if err != nil {
		log.Fatal(err)
	}

	// Progress from inside a Parallel is reported under the parallel's name.
	checks := v2.NewParallel(v2.WithParallelName("checks"), v2.WithParallelSteps(
		v2.ExecutorFunc(func(in *v2.StepInput) (*v2.StepOutput, error) {
			in.ReportProgress("schema check ok", nil)
			return &v2.StepOutput{Content: "schema ok"}, nil
		}),
	))

	wf := v2.NewWorkflow(
		v2.WithWorkflowName("progress demo"),
		v2.WithWorkflowSteps([]interface{}{ingest, checks, publish}),
	)
	wf.OnEvent(v2.StepProgressEvent, func(e *v2.WorkflowRunResponseEvent) {
		p := e.Data.(*v2.StepProgress)
		fmt.Printf("[progress] step=%v index=%v attempt=%v: %s %v\n",
			e.Metadata["step_name"], e.Metadata["step_index"], e.Metadata["attempt"], p.Content, p.Data)
	})
	wf.OnEvent(v2.StepCompletedEvent, func(e *v2.WorkflowRunResponseEvent) {
		fmt.Printf("[completed] %v\n", e.Metadata["step_name"])
	})

	resp, err := wf.Run(context.Background(), "nightly sync")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("result:", resp.Content)
}

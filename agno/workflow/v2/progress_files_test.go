package v2

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/guardrails"
	"github.com/devalexandre/agno-golang/agno/models"
)

type progressRecorder struct {
	mu     sync.Mutex
	events []*WorkflowRunResponseEvent
	order  []WorkflowRunEvent
}

func (r *progressRecorder) attach(w *Workflow) {
	for _, ev := range []WorkflowRunEvent{StepStartedEvent, StepProgressEvent, StepCompletedEvent} {
		ev := ev
		w.OnEvent(ev, func(e *WorkflowRunResponseEvent) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.order = append(r.order, ev)
			if ev == StepProgressEvent {
				r.events = append(r.events, e)
			}
		})
	}
}

func TestStepProgressEmittedFromFunctionExecutor(t *testing.T) {
	ingest, _ := NewStep(WithName("ingest"), WithExecutor(func(in *StepInput) (*StepOutput, error) {
		in.ReportProgress("half way", map[string]interface{}{"done": 1})
		in.ReportProgress("done", nil)
		return &StepOutput{Content: "ok"}, nil
	}))
	tests := []struct {
		name  string
		steps WorkflowSteps
		run   func(*Workflow) error
	}{
		{"step sequence", []*Step{ingest}, func(w *Workflow) error { _, err := w.Run(context.Background(), "x"); return err }},
		{"interface sequence", []interface{}{ingest}, func(w *Workflow) error { _, err := w.Run(context.Background(), "x"); return err }},
		{"stream", []*Step{ingest}, func(w *Workflow) error { _, err := w.RunWithStream(context.Background(), "x", true); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := NewWorkflow(WithWorkflowSteps(tt.steps))
			rec := &progressRecorder{}
			rec.attach(wf)
			if err := tt.run(wf); err != nil {
				t.Fatal(err)
			}
			if len(rec.events) != 2 {
				t.Fatalf("expected 2 progress events, got %d", len(rec.events))
			}
			first := rec.events[0]
			p, ok := first.Data.(*StepProgress)
			if !ok || p.Content != "half way" || p.Data["done"] != 1 {
				t.Errorf("unexpected progress payload: %#v", first.Data)
			}
			if first.Metadata["step_name"] != "ingest" || first.Metadata["step_index"] != 0 || first.Metadata["attempt"] != 1 {
				t.Errorf("unexpected metadata: %#v", first.Metadata)
			}
			if rec.order[len(rec.order)-1] != StepCompletedEvent {
				t.Errorf("progress must precede StepCompleted, got order %v", rec.order)
			}
		})
	}
}

func TestStepProgressNestedAndRetries(t *testing.T) {
	calls := 0
	flaky, _ := NewStep(WithName("flaky"), WithMaxRetries(1), WithExecutor(func(in *StepInput) (*StepOutput, error) {
		calls++
		in.ReportProgress("trying", nil)
		if calls == 1 {
			return nil, errors.New("transient")
		}
		return &StepOutput{Content: "ok"}, nil
	}))
	raw := func(in *StepInput) (*StepOutput, error) {
		in.ReportProgress("raw", nil)
		return &StepOutput{Content: "raw"}, nil
	}
	par := NewParallel(WithParallelName("fan"), WithParallelSteps(ExecutorFunc(raw)))
	cond := NewCondition(WithConditionName("gate"), WithIf(func(*StepInput) bool { return true }), WithThen(flaky))
	wf := NewWorkflow(WithWorkflowSteps([]interface{}{par, cond}))
	rec := &progressRecorder{}
	rec.attach(wf)
	if _, err := wf.Run(context.Background(), "x"); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 3 {
		t.Fatalf("expected 3 progress events, got %d", len(rec.events))
	}
	type want struct {
		step    string
		index   int
		attempt int
	}
	wants := []want{{"fan", 0, 1}, {"flaky", 1, 1}, {"flaky", 1, 2}}
	for i, w := range wants {
		md := rec.events[i].Metadata
		if md["step_name"] != w.step || md["step_index"] != w.index || md["attempt"] != w.attempt {
			t.Errorf("event %d: got %v, want %+v", i, md, w)
		}
	}
}

func TestReportProgressWithoutWorkflowIsNoop(t *testing.T) {
	(&StepInput{}).ReportProgress("x", nil)
	var nilInput *StepInput
	nilInput.ReportProgress("x", nil)
}

func file(name string) models.File {
	return models.File{Name: name, MimeType: "text/plain", Data: []byte(name)}
}

func fileNames(files []models.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

func equalNames(got []models.File, want ...string) bool {
	names := fileNames(got)
	if len(names) != len(want) {
		return false
	}
	for i := range names {
		if names[i] != want[i] {
			return false
		}
	}
	return true
}

// producer returns an executor that emits a file and records the files it saw.
func producer(name string, seen *[]string) ExecutorFunc {
	return func(in *StepInput) (*StepOutput, error) {
		if seen != nil {
			*seen = fileNames(in.Files)
		}
		return &StepOutput{Content: name, Files: []models.File{file(name)}}, nil
	}
}

func TestFilesPropagateThroughContainers(t *testing.T) {
	input := &StepInput{Message: "x", Files: []models.File{file("in")}}

	t.Run("parallel aggregates in declaration order", func(t *testing.T) {
		p := NewParallel(WithParallelName("p"), WithParallelSteps(producer("a", nil), producer("b", nil), ExecutorFunc(func(*StepInput) (*StepOutput, error) {
			return &StepOutput{Content: "none"}, nil
		})))
		out, err := p.Execute(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if !equalNames(out.Files, "a", "b") {
			t.Errorf("got %v", fileNames(out.Files))
		}
	})

	t.Run("condition chains and aggregates", func(t *testing.T) {
		var seen []string
		c := NewCondition(WithConditionName("c"), WithIf(func(*StepInput) bool { return true }),
			WithThen(producer("a", nil), producer("b", &seen)))
		out, err := c.Execute(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if !equalNames(out.Files, "a", "b") || len(seen) != 2 || seen[0] != "in" || seen[1] != "a" {
			t.Errorf("out=%v seen=%v", fileNames(out.Files), seen)
		}
	})

	t.Run("router chains and aggregates", func(t *testing.T) {
		var seen []string
		r := NewRouter(WithRouterName("r"), WithRouteFunc(func(*StepInput) string { return "go" }),
			WithRoute("go", producer("a", nil), producer("b", &seen)))
		out, err := r.Execute(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if !equalNames(out.Files, "a", "b") || len(seen) != 2 || seen[1] != "a" {
			t.Errorf("out=%v seen=%v", fileNames(out.Files), seen)
		}
	})

	t.Run("loop chains across iterations", func(t *testing.T) {
		var seen []string
		i := 0
		l := NewLoop(WithLoopName("l"), WithLoopCondition(ForN(2)), WithMaxIterations(5),
			WithLoopSteps(ExecutorFunc(func(in *StepInput) (*StepOutput, error) {
				i++
				seen = fileNames(in.Files)
				return &StepOutput{Content: "it", Files: []models.File{file("it" + string(rune('0'+i)))}}, nil
			})))
		out, err := l.Execute(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if !equalNames(out.Files, "it1", "it2") || len(seen) != 2 || seen[0] != "in" || seen[1] != "it1" {
			t.Errorf("out=%v seen=%v", fileNames(out.Files), seen)
		}
	})

	t.Run("steps chains and aggregates", func(t *testing.T) {
		var seen []string
		s := NewSteps(WithStepsName("s"), WithStepsList(producer("a", nil), producer("b", &seen)))
		out, err := s.Execute(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if !equalNames(out.Files, "a", "b") || len(seen) != 2 {
			t.Errorf("out=%v seen=%v", fileNames(out.Files), seen)
		}
	})
}

func TestWorkflowFilesReachNextStepAndResponse(t *testing.T) {
	var seen []string
	par := NewParallel(WithParallelName("p"), WithParallelSteps(producer("a", nil)))
	last, _ := NewStep(WithName("last"), WithExecutor(producer("b", &seen)))
	wf := NewWorkflow(WithWorkflowSteps([]interface{}{par, last}))
	resp, err := wf.Run(context.Background(), &WorkflowExecutionInput{Message: "x", Files: []models.File{file("in")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "in" || seen[1] != "a" {
		t.Errorf("next step saw %v", seen)
	}
	if !equalNames(resp.Files, "in", "a", "b") {
		t.Errorf("response files %v", fileNames(resp.Files))
	}
}

func TestStepStopsRetryingOnGuardrailRejection(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		skip      bool
		wantCalls int
		wantErr   bool
	}{
		{"input check error", guardrails.NewInputCheckError("blocked", "", nil), false, 1, true},
		{"output check error wrapped", errors.Join(errors.New("ctx"), guardrails.NewOutputCheckError("bad", "", nil)), false, 1, true},
		{"check error honours skip", guardrails.NewInputCheckError("blocked", "", nil), true, 1, false},
		{"ordinary error retries", errors.New("transient"), false, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			step, _ := NewStep(WithName("s"), WithMaxRetries(1), WithSkipOnFailure(tt.skip),
				WithExecutor(func(*StepInput) (*StepOutput, error) {
					calls++
					return nil, tt.err
				}))
			_, err := step.Execute(context.Background(), &StepInput{})
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

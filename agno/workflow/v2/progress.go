package v2

import (
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
)

// StepProgressEvent is emitted when a function executor reports progress
// through StepInput.ReportProgress while it is still running.
const StepProgressEvent WorkflowRunEvent = "StepProgress"

// StepProgress is an observer update emitted by a function executor while it
// runs. It never replaces the step output: it only surfaces as a StepProgress
// event in the workflow event stream.
type StepProgress struct {
	Content string                 `json:"content,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// progressSink receives progress reported by an executor together with the
// name of the step reporting it and the 1-based retry attempt.
type progressSink func(stepName string, attempt int, progress StepProgress)

// ReportProgress emits a StepProgress event for the step currently executing
// with this input. data is optional. It is a no-op when the input is not
// attached to a running workflow, so executors can call it unconditionally.
//
// Example:
//
//	func ingest(input *v2.StepInput) (*v2.StepOutput, error) {
//		for i, batch := range batches {
//			input.ReportProgress(fmt.Sprintf("batch %d/%d", i+1, len(batches)), map[string]interface{}{"done": i + 1})
//			...
//		}
//		return &v2.StepOutput{Content: "ok"}, nil
//	}
func (s *StepInput) ReportProgress(content string, data map[string]interface{}) {
	if s == nil || s.progress == nil {
		return
	}
	attempt := s.attempt
	if attempt < 1 {
		attempt = 1
	}
	s.progress(s.progressStep, attempt, StepProgress{Content: content, Data: data})
}

// inheritRuntime copies the run-scoped state that is not part of the public
// literal fields (progress reporting) plus the files from src into s. Container
// steps call it on every StepInput they derive from the one they received.
func (s *StepInput) inheritRuntime(src *StepInput) *StepInput {
	if src == nil {
		return s
	}
	s.progress = src.progress
	s.progressStep = src.progressStep
	s.attempt = src.attempt
	if len(src.Files) > 0 {
		s.Files = append([]models.File(nil), src.Files...)
	}
	return s
}

// forStep returns a shallow copy of the input scoped to a named step and
// retry attempt, so progress events are attributed to the right step.
func (s *StepInput) forStep(name string, attempt int) *StepInput {
	if s == nil {
		return nil
	}
	cp := *s
	if name != "" {
		cp.progressStep = name
	}
	cp.attempt = attempt
	return &cp
}

// appendFiles returns dst extended with the files of every non-nil output.
func appendFiles(dst []models.File, outputs ...*StepOutput) []models.File {
	for _, out := range outputs {
		if out != nil && len(out.Files) > 0 {
			dst = append(dst, out.Files...)
		}
	}
	return dst
}

// attachProgress wires the workflow event stream into a step input for the
// top-level step at index idx.
func (w *Workflow) attachProgress(input *StepInput, stepName string, idx int) {
	input.progressStep = stepName
	input.attempt = 0
	input.progress = func(name string, attempt int, p StepProgress) {
		if name == "" {
			name = stepName
		}
		w.emitEvent(&WorkflowRunResponseEvent{
			Event:     StepProgressEvent,
			Timestamp: time.Now(),
			Data:      &p,
			Metadata: map[string]interface{}{
				"step_name":  name,
				"step_index": idx,
				"attempt":    attempt,
				"content":    p.Content,
				"data":       p.Data,
			},
		})
	}
}

package v2

import (
	"fmt"

	"github.com/devalexandre/agno-golang/agno/models"
)

// resetRunFiles clears the files collected by a previous run.
func (w *Workflow) resetRunFiles() {
	w.mu.Lock()
	w.runFiles = nil
	w.mu.Unlock()
}

// recordFiles remembers the files produced by a top-level step output so the
// run response can expose them.
func (w *Workflow) recordFiles(output *StepOutput) {
	if output == nil || len(output.Files) == 0 {
		return
	}
	w.mu.Lock()
	w.runFiles = append(w.runFiles, output.Files...)
	w.mu.Unlock()
}

// collectRunFiles returns the input files followed by the produced files.
func (w *Workflow) collectRunFiles(execInput *WorkflowExecutionInput) []models.File {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var files []models.File
	if execInput != nil {
		files = append(files, execInput.Files...)
	}
	files = append(files, w.runFiles...)
	if len(files) == 0 {
		return nil
	}
	return files
}

// stepItemName returns the name a step item will be reported under before it
// runs. It mirrors the names used for StepStarted/StepCompleted events.
func stepItemName(item interface{}, index int) string {
	name := ""
	switch v := item.(type) {
	case *Step:
		name = v.Name
	case *Steps:
		name = v.Name
	case *Loop:
		name = v.Name
	case *Parallel:
		name = v.Name
	case *Condition:
		name = v.Name
	case *Router:
		name = v.Name
	}
	if name == "" {
		name = fmt.Sprintf("step_%d", index)
	}
	return name
}

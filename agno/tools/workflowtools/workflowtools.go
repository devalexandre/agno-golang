// Package workflowtools ports Agno's WorkflowTools: lets an agent run a
// workflow (plus optional think/analyze scratchpad tools).
//
// Differences from Python:
//   - Python's run_workflow takes a nested `input` object
//     ({input_data, additional_data}); the Go schema generator does not describe
//     nested objects, so the two fields are top-level parameters here.
//   - workflow/v2's Run has no additional_data argument; with NewWorkflowTools
//     it is ignored. Use NewWorkflowToolsFromFunc to handle it yourself.
package workflowtools

import (
	"context"
	"encoding/json"
	"fmt"

	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"

	"github.com/devalexandre/agno-golang/agno/tools/internal/scratchpad"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// RunFunc executes a workflow and returns a JSON-serialisable result.
type RunFunc func(ctx context.Context, inputData string, additionalData map[string]interface{}) (interface{}, error)

// WorkflowTools exposes RunWorkflow, Think and Analyze.
type WorkflowTools struct {
	toolkit.Toolkit
	Instructions    string
	AddInstructions bool

	run                       RunFunc
	enRun, enThink, enAnalyze bool
	addFewShot                bool
	fewShot                   string
	custom                    *string
	state                     *scratchpad.State
}

// Option configures WorkflowTools.
type Option func(*WorkflowTools)

// WithRunWorkflow enables or disables RunWorkflow (default true).
func WithRunWorkflow(b bool) Option { return func(w *WorkflowTools) { w.enRun = b } }

// WithThink enables or disables Think (default false).
func WithThink(b bool) Option { return func(w *WorkflowTools) { w.enThink = b } }

// WithAnalyze enables or disables Analyze (default false).
func WithAnalyze(b bool) Option { return func(w *WorkflowTools) { w.enAnalyze = b } }

// WithAll enables every tool.
func WithAll() Option {
	return func(w *WorkflowTools) { w.enRun, w.enThink, w.enAnalyze = true, true, true }
}

// WithInstructions replaces the default instructions.
func WithInstructions(s string) Option { return func(w *WorkflowTools) { w.custom = &s } }

// WithAddInstructions controls whether instructions are added to the prompt (default true).
func WithAddInstructions(b bool) Option { return func(w *WorkflowTools) { w.AddInstructions = b } }

// WithFewShot appends few-shot examples (Python has no default examples here).
func WithFewShot(examples string) Option {
	return func(w *WorkflowTools) { w.addFewShot, w.fewShot = true, examples }
}

// WithSessionState backs the logs/results with the given map.
func WithSessionState(m map[string]interface{}) Option {
	return func(w *WorkflowTools) { w.state = scratchpad.New(m) }
}

// RunWorkflowParams are the parameters of RunWorkflow.
type RunWorkflowParams struct {
	InputData      string                 `json:"input_data" description:"The input data for the workflow." required:"true"`
	AdditionalData map[string]interface{} `json:"additional_data,omitempty" description:"The additional data for the workflow."`
}

// ThinkParams are the parameters of Think.
type ThinkParams struct {
	Thought string `json:"thought" description:"Your thought process and reasoning about workflow execution." required:"true"`
}

// AnalyzeParams are the parameters of Analyze.
type AnalyzeParams struct {
	Analysis string `json:"analysis" description:"Your analysis of the workflow execution results." required:"true"`
}

// NewWorkflowTools wraps a workflow/v2 Workflow.
func NewWorkflowTools(wf *v2.Workflow, opts ...Option) (*WorkflowTools, error) {
	if wf == nil {
		return nil, fmt.Errorf("workflow must be provided when using WorkflowTools")
	}
	return NewWorkflowToolsFromFunc(func(ctx context.Context, in string, _ map[string]interface{}) (interface{}, error) {
		return wf.Run(ctx, in)
	}, opts...)
}

// NewWorkflowToolsFromFunc builds the toolkit around any run function.
func NewWorkflowToolsFromFunc(run RunFunc, opts ...Option) (*WorkflowTools, error) {
	if run == nil {
		return nil, fmt.Errorf("run function must not be nil")
	}
	w := &WorkflowTools{run: run, enRun: true, AddInstructions: true}
	for _, o := range opts {
		o(w)
	}
	if w.state == nil {
		w.state = scratchpad.New(nil)
	}
	if w.custom != nil {
		w.Instructions = *w.custom
	} else {
		w.Instructions = DefaultInstructions
		if w.addFewShot && w.fewShot != "" {
			w.Instructions += "\n" + w.fewShot
		}
	}
	w.Toolkit = toolkit.NewToolkit()
	w.Toolkit.Name = "workflow_tools"
	w.Toolkit.Description = "Run a workflow and reason about its results."
	// Workflows usually run models; allow them more time than plain HTTP calls.
	w.Toolkit.Timeout = 0
	if w.enThink {
		w.Toolkit.Register("Think", "Use this tool as a scratchpad to reason about the workflow execution, refine your approach, brainstorm workflow inputs, or revise your plan. Call Think whenever you need to figure out what to do next, analyze the user's requirements, plan workflow inputs, or decide on execution strategy.", w, w.Think, ThinkParams{})
	}
	if w.enRun {
		w.Toolkit.Register("RunWorkflow", "Use this tool to execute the workflow with the specified inputs and parameters. After thinking through the requirements, use this tool to run the workflow with appropriate inputs.", w, w.RunWorkflow, RunWorkflowParams{})
	}
	if w.enAnalyze {
		w.Toolkit.Register("Analyze", "Use this tool to evaluate whether the workflow execution results are correct and sufficient. If not, go back to Think or Run with refined inputs or parameters.", w, w.Analyze, AnalyzeParams{})
	}
	return w, nil
}

// Think logs a thought and returns every thought so far.
func (w *WorkflowTools) Think(p ThinkParams) (interface{}, error) {
	return w.state.AppendLog("workflow_thoughts", "Workflow Thoughts", p.Thought), nil
}

// RunWorkflow runs the workflow and returns its output as JSON.
func (w *WorkflowTools) RunWorkflow(p RunWorkflowParams) (interface{}, error) {
	ctx := context.Background()
	if w.Toolkit.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.Toolkit.Timeout)
		defer cancel()
	}
	result, err := w.run(ctx, p.InputData, p.AdditionalData)
	if err != nil {
		return fmt.Sprintf("Error running workflow: %v", err), nil
	}
	w.state.AppendValue("workflow_results", result)
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error running workflow: %v", err), nil
	}
	return string(b), nil
}

// Analyze logs an analysis and returns every analysis so far.
func (w *WorkflowTools) Analyze(p AnalyzeParams) (interface{}, error) {
	return w.state.AppendLog("workflow_analysis", "Workflow Analysis", p.Analysis), nil
}

// SessionState returns the logs and workflow results.
func (w *WorkflowTools) SessionState() map[string]interface{} { return w.state.Snapshot() }

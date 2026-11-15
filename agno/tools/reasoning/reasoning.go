// Package reasoning ports Agno's ReasoningTools: a think/analyze scratchpad
// that lets a model reason step by step through tool calls.
//
// Python keeps the steps in run_context.session_state["reasoning_steps"][run_id].
// Go tool methods receive no run context, so the steps live on the toolkit
// instance (one log per toolkit; call Reset between runs, or use one toolkit
// per agent run). SessionState returns them in the same shape as Python.
package reasoning

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// NextAction mirrors agno.reasoning.step.NextAction.
type NextAction string

const (
	NextActionContinue    NextAction = "continue"
	NextActionValidate    NextAction = "validate"
	NextActionFinalAnswer NextAction = "final_answer"
	NextActionReset       NextAction = "reset"
)

// ReasoningStep mirrors agno.reasoning.step.ReasoningStep.
type ReasoningStep struct {
	Title      string     `json:"title,omitempty"`
	Action     string     `json:"action,omitempty"`
	Result     string     `json:"result,omitempty"`
	Reasoning  string     `json:"reasoning,omitempty"`
	NextAction NextAction `json:"next_action,omitempty"`
	Confidence float64    `json:"confidence,omitempty"`
}

// ReasoningTools provides the Think and Analyze tools.
type ReasoningTools struct {
	toolkit.Toolkit
	// Instructions are the toolkit instructions (Python: instructions).
	Instructions string
	// AddInstructions reports whether the instructions should be added to the
	// system prompt (Python: add_instructions, default false).
	AddInstructions bool

	enableThink, enableAnalyze bool
	addFewShot                 bool
	fewShotExamples            string
	customInstructions         *string

	mu    sync.Mutex
	steps []ReasoningStep
}

// Option configures ReasoningTools.
type Option func(*ReasoningTools)

// WithThink enables or disables Think (default true).
func WithThink(enabled bool) Option { return func(r *ReasoningTools) { r.enableThink = enabled } }

// WithAnalyze enables or disables Analyze (default true).
func WithAnalyze(enabled bool) Option { return func(r *ReasoningTools) { r.enableAnalyze = enabled } }

// WithAll enables every tool.
func WithAll() Option { return func(r *ReasoningTools) { r.enableThink, r.enableAnalyze = true, true } }

// WithInstructions replaces the default instructions.
func WithInstructions(s string) Option { return func(r *ReasoningTools) { r.customInstructions = &s } }

// WithAddInstructions controls whether instructions are added to the prompt.
func WithAddInstructions(add bool) Option { return func(r *ReasoningTools) { r.AddInstructions = add } }

// WithFewShot appends few-shot examples (examples == "" uses the defaults).
func WithFewShot(examples string) Option {
	return func(r *ReasoningTools) { r.addFewShot, r.fewShotExamples = true, examples }
}

// ThinkParams are the parameters of Think.
type ThinkParams struct {
	Title      string  `json:"title" description:"A concise title for this step" required:"true"`
	Thought    string  `json:"thought" description:"Your detailed thought for this step" required:"true"`
	Action     string  `json:"action,omitempty" description:"What you'll do based on this thought"`
	Confidence float64 `json:"confidence,omitempty" description:"How confident you are about this thought (0.0 to 1.0). Default 0.8 (0 or omitted uses the default)"`
}

// AnalyzeParams are the parameters of Analyze.
type AnalyzeParams struct {
	Title      string  `json:"title" description:"A concise title for this analysis step" required:"true"`
	Result     string  `json:"result" description:"The outcome of the previous action" required:"true"`
	Analysis   string  `json:"analysis" description:"Your analysis of the results" required:"true"`
	NextAction string  `json:"next_action,omitempty" description:"What to do next (\"continue\", \"validate\", or \"final_answer\"). Default continue"`
	Confidence float64 `json:"confidence,omitempty" description:"How confident you are in this analysis (0.0 to 1.0). Default 0.8 (0 or omitted uses the default)"`
}

// NewReasoningTools creates the reasoning toolkit.
func NewReasoningTools(opts ...Option) *ReasoningTools {
	r := &ReasoningTools{enableThink: true, enableAnalyze: true}
	for _, o := range opts {
		o(r)
	}
	if r.customInstructions != nil {
		r.Instructions = *r.customInstructions
	} else {
		r.Instructions = "<reasoning_instructions>\n" + DefaultInstructions
		if r.addFewShot {
			ex := r.fewShotExamples
			if ex == "" {
				ex = FewShotExamples
			}
			r.Instructions += "\n" + ex
		}
		r.Instructions += "\n</reasoning_instructions>\n"
	}

	r.Toolkit = toolkit.NewToolkit()
	r.Toolkit.Name = "reasoning_tools"
	r.Toolkit.Description = "Step-by-step reasoning tools: think and analyze."
	if r.enableThink {
		r.Toolkit.Register("Think", "Use this tool as a scratchpad to reason about the question and work through it step-by-step. This tool will help you break down complex problems into logical steps and track the reasoning process. You can call it as many times as needed. These internal thoughts are never revealed to the user. Returns a list of previous thoughts and the new thought.", r, r.Think, ThinkParams{})
	}
	if r.enableAnalyze {
		r.Toolkit.Register("Analyze", "Use this tool to analyze results from a reasoning step and determine next actions. Returns a list of previous thoughts and the new analysis.", r, r.Analyze, AnalyzeParams{})
	}
	return r
}

func confidenceOr(c float64) float64 {
	if c <= 0 {
		return 0.8
	}
	return c
}

// Think records a reasoning step and returns every step so far.
func (r *ReasoningTools) Think(p ThinkParams) (interface{}, error) {
	return r.record(ReasoningStep{
		Title:      p.Title,
		Reasoning:  p.Thought,
		Action:     p.Action,
		NextAction: NextActionContinue,
		Confidence: confidenceOr(p.Confidence),
	}), nil
}

// Analyze records an analysis step and returns every step so far.
func (r *ReasoningTools) Analyze(p AnalyzeParams) (interface{}, error) {
	next := NextActionContinue
	switch strings.ToLower(p.NextAction) {
	case "validate":
		next = NextActionValidate
	case "final", "final_answer", "finalize":
		next = NextActionFinalAnswer
	}
	return r.record(ReasoningStep{
		Title:      p.Title,
		Result:     p.Result,
		Reasoning:  p.Analysis,
		NextAction: next,
		Confidence: confidenceOr(p.Confidence),
	}), nil
}

func (r *ReasoningTools) record(step ReasoningStep) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, step)
	var sb strings.Builder
	for i, s := range r.steps {
		action := s.Action
		if action == "" {
			action = "None"
		}
		fmt.Fprintf(&sb, "Step %d:\nTitle: %s\nReasoning: %s\nAction: %s\nConfidence: %s\n\n",
			i+1, s.Title, s.Reasoning, action, formatConfidence(s.Confidence))
	}
	return strings.TrimSpace(sb.String())
}

// formatConfidence prints floats the way Python's str(float) does (0.8, 1.0).
func formatConfidence(c float64) string {
	s := fmt.Sprintf("%g", c)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}

// Steps returns a copy of the recorded reasoning steps.
func (r *ReasoningTools) Steps() []ReasoningStep {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ReasoningStep(nil), r.steps...)
}

// SessionState returns {"reasoning_steps": [json, ...]} like Python's session state entry.
func (r *ReasoningTools) SessionState() map[string]interface{} {
	steps := r.Steps()
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		b, _ := json.Marshal(s)
		out = append(out, string(b))
	}
	return map[string]interface{}{"reasoning_steps": out}
}

// Reset clears the recorded steps (call between runs).
func (r *ReasoningTools) Reset() {
	r.mu.Lock()
	r.steps = nil
	r.mu.Unlock()
}

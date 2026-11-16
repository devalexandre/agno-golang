package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/guardrails"
	"github.com/devalexandre/agno-golang/agno/models"
)

// HookArgs is what a pre/post run hook receives (Python passes run_input /
// run_output, agent, session, user_id and metadata).
type HookArgs struct {
	// Agent running the hook.
	Agent *Agent
	// RunID, SessionID and UserID of the run.
	RunID     string
	SessionID string
	UserID    string
	// Input is the mutable run input (pre-hooks). Rewriting Input.Content
	// replaces the prompt when the run input is a string.
	Input *guardrails.Input
	// RawInput is the original input value passed to Run.
	RawInput interface{}
	// Output is the run output (post-hooks); hooks may modify it in place.
	Output *models.RunResponse
	// Metadata carries the run metadata.
	Metadata map[string]interface{}
}

// Hook is a pre/post run hook (Python pre_hooks / post_hooks entry).
//
// Semantics follow Python: a hook returning a *guardrails.CheckError (input or
// output check error) aborts the run with that error; any other error is
// logged and the run continues.
type Hook interface {
	RunHook(ctx context.Context, args *HookArgs) error
}

// HookFunc adapts a function to the Hook interface.
type HookFunc func(ctx context.Context, args *HookArgs) error

// RunHook implements Hook.
func (f HookFunc) RunHook(ctx context.Context, args *HookArgs) error { return f(ctx, args) }

// hookNamer is implemented by hooks exposing a name for logs.
type hookNamer interface{ GetName() string }

func hookName(h Hook, i int) string {
	if n, ok := h.(hookNamer); ok {
		return n.GetName()
	}
	return fmt.Sprintf("#%d", i+1)
}

// guardrailHook runs a guardrail as a pre-hook (on the input) or a post-hook
// (on the output). Any rejection is reported as a CheckError, so it blocks.
type guardrailHook struct {
	g guardrails.Guardrail
}

// GuardrailHook adapts a guardrail to a Hook, so it can be used in
// PreRunHooks / PostRunHooks like Python guardrails in pre_hooks. Guardrails
// implementing guardrails.InputChecker / OutputChecker may rewrite the content
// (e.g. PII masking).
func GuardrailHook(g guardrails.Guardrail) Hook { return &guardrailHook{g: g} }

// GuardrailHooks adapts several guardrails.
func GuardrailHooks(gs ...guardrails.Guardrail) []Hook {
	out := make([]Hook, 0, len(gs))
	for _, g := range gs {
		out = append(out, GuardrailHook(g))
	}
	return out
}

func (h *guardrailHook) GetName() string { return h.g.GetName() }

func (h *guardrailHook) RunHook(ctx context.Context, args *HookArgs) error {
	var err error
	stage := guardrails.StageInput
	switch {
	case args.Output != nil:
		stage = guardrails.StageOutput
		if oc, ok := h.g.(guardrails.OutputChecker); ok {
			out := guardrails.NewOutput(args.Output.TextContent)
			if err = oc.CheckOutput(ctx, out); err == nil {
				args.Output.TextContent = out.Content
			}
		} else {
			err = h.g.Check(ctx, guardrails.NewOutput(args.Output.TextContent))
		}
	case args.Input != nil:
		if ic, ok := h.g.(guardrails.InputChecker); ok {
			err = ic.CheckInput(ctx, args.Input)
		} else {
			err = h.g.Check(ctx, args.Input)
		}
	}
	if err == nil {
		return nil
	}
	if ce, ok := guardrails.AsCheckError(err); ok {
		if ce.Guardrail == "" {
			return ce.WithGuardrail(h.g.GetName())
		}
		return ce
	}
	// A guardrail rejection always blocks the run.
	if stage == guardrails.StageOutput {
		return guardrails.NewOutputCheckError(err.Error(), guardrails.TriggerValidationFailed, nil).WithGuardrail(h.g.GetName())
	}
	return guardrails.NewInputCheckError(err.Error(), guardrails.TriggerValidationFailed, nil).WithGuardrail(h.g.GetName())
}

// backgroundHook runs the wrapped hook in a goroutine (Python
// @hook(run_in_background=True)). It never blocks or aborts the run.
type backgroundHook struct {
	h Hook
}

// BackgroundHook runs h asynchronously with a copy of the hook arguments.
// Errors are logged. Guardrails should not be run in background: they could
// not block the run.
func BackgroundHook(h Hook) Hook { return &backgroundHook{h: h} }

func (b *backgroundHook) RunHook(ctx context.Context, args *HookArgs) error {
	cp := *args
	if args.Input != nil {
		in := *args.Input
		cp.Input = &in
	}
	if args.Output != nil {
		out := *args.Output
		cp.Output = &out
	}
	ctx = context.WithoutCancel(ctx)
	go func() {
		if err := b.h.RunHook(ctx, &cp); err != nil {
			log.Printf("Warning: background hook failed: %v", err)
		}
	}()
	return nil
}

// runHooks executes hooks in order with Python semantics.
func (a *Agent) runHooks(kind string, hooks []Hook, args *HookArgs) error {
	for i, h := range hooks {
		if h == nil {
			continue
		}
		if err := h.RunHook(a.ctx, args); err != nil {
			if _, ok := guardrails.AsCheckError(err); ok {
				return err
			}
			log.Printf("Warning: %s-hook %s execution failed: %v", kind, hookName(h, i), err)
		}
	}
	return nil
}

func (a *Agent) hookArgs() *HookArgs {
	args := &HookArgs{Agent: a, SessionID: a.sessionID, UserID: a.userID}
	if a.currentRun != nil {
		args.RunID = a.currentRun.id
	}
	return args
}

// runPreRunHooks executes PreRunHooks and returns the (possibly rewritten) input.
func (a *Agent) runPreRunHooks(input interface{}, metadata map[string]interface{}) (interface{}, error) {
	if len(a.preRunHooks) == 0 {
		return input, nil
	}
	args := a.hookArgs()
	args.RawInput = input
	args.Metadata = metadata
	text, isString := input.(string)
	if !isString {
		text, _ = guardrails.ExtractText(input)
	}
	args.Input = guardrails.NewInput(text)
	if err := a.runHooks("pre", a.preRunHooks, args); err != nil {
		return input, err
	}
	if isString {
		return args.Input.Content, nil
	}
	return input, nil
}

// runPostRunHooks executes PostRunHooks on the final response.
func (a *Agent) runPostRunHooks(resp *models.RunResponse) error {
	if len(a.postRunHooks) == 0 {
		return nil
	}
	args := a.hookArgs()
	args.Output = resp
	return a.runHooks("post", a.postRunHooks, args)
}

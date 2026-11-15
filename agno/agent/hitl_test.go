package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/guardrails"
	"github.com/devalexandre/agno-golang/agno/models"
	inmemory "github.com/devalexandre/agno-golang/agno/storage/in_memory"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// toolCallingModel behaves like the real model clients: on a user turn it
// emits the planned tool calls, executes them through the tools passed with
// models.WithTools and calls itself again (a new "HTTP request", which fails
// when ctx is cancelled). When the last message is a tool result it answers
// with a summary of the tool messages.
type toolCallingModel struct {
	mu          sync.Mutex
	planned     []tools.FunctionCall
	invocations int
	lastMsgs    []models.Message
	seenSchemas map[string]map[string]interface{}
}

func (m *toolCallingModel) Invoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (*models.MessageResponse, error) {
	m.mu.Lock()
	m.invocations++
	m.lastMsgs = msgs
	m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	co := &models.CallOptions{}
	for _, o := range opts {
		o(co)
	}
	if m.seenSchemas == nil {
		m.seenSchemas = map[string]map[string]interface{}{}
	}
	for _, t := range co.ToolCall {
		for name, meth := range t.GetMethods() {
			m.seenSchemas[name] = t.GetParameterStruct(name)
			_ = meth
		}
	}

	if last := msgs[len(msgs)-1]; last.Role == models.TypeToolRole {
		var parts []string
		for _, msg := range msgs {
			if msg.Role == models.TypeToolRole {
				parts = append(parts, msg.Content)
			}
		}
		return &models.MessageResponse{Role: "assistant", Model: "fake", Content: "final: " + strings.Join(parts, " | ")}, nil
	}
	if len(m.planned) == 0 {
		return &models.MessageResponse{Role: "assistant", Model: "fake", Content: "no tools"}, nil
	}

	assistant := models.Message{Role: models.TypeAssistantRole}
	var results []models.Message
	for i, fc := range m.planned {
		id := fmt.Sprintf("call_%d", i)
		assistant.ToolCalls = append(assistant.ToolCalls, tools.ToolCall{ID: id, Type: "function", Function: fc})
		content := "Tool not found"
		for _, t := range co.ToolCall {
			if _, ok := t.GetMethods()[fc.Name]; ok {
				res, err := t.Execute(fc.Name, json.RawMessage(fc.Arguments))
				if err != nil {
					content = "Error executing tool: " + err.Error()
				} else {
					content = fmt.Sprint(res)
				}
			}
		}
		results = append(results, models.Message{Role: models.TypeToolRole, Content: content, ToolCallID: &id})
	}
	next := append(append(append([]models.Message(nil), msgs...), assistant), results...)
	return m.Invoke(ctx, next, opts...)
}

func (m *toolCallingModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (m *toolCallingModel) InvokeStream(context.Context, []models.Message, ...models.Option) error {
	return nil
}
func (m *toolCallingModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (m *toolCallingModel) GetID() string { return "fake" }

// --- test toolkit ---

type sendParams struct {
	To      string `json:"to" description:"Recipient" required:"true"`
	Subject string `json:"subject" description:"Subject" required:"true"`
}

type opsTool struct {
	toolkit.Toolkit
	mu    sync.Mutex
	calls []string
}

func newOpsTool(opts map[string][]toolkit.MethodOption) *opsTool {
	t := &opsTool{Toolkit: toolkit.NewToolkit()}
	t.Name = "Ops"
	t.Description = "operations"
	t.RegisterWithOptions("Send", "Send an email", t, t.Send, sendParams{}, opts["Send"]...)
	t.RegisterWithOptions("Lookup", "Lookup data", t, t.Lookup, sendParams{}, opts["Lookup"]...)
	return t
}

func (t *opsTool) Send(p sendParams) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls = append(t.calls, "send:"+p.To+":"+p.Subject)
	return "sent to " + p.To, nil
}

func (t *opsTool) Lookup(p sendParams) (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.calls = append(t.calls, "lookup:"+p.To)
	return "found " + p.To, nil
}

func (t *opsTool) called() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.calls...)
}

func newHITLAgent(t *testing.T, m models.AgnoModelInterface, tool toolkit.Tool, mutate func(*AgentConfig)) *Agent {
	t.Helper()
	cfg := AgentConfig{Model: m, Tools: []toolkit.Tool{tool}, Name: "hitl"}
	if mutate != nil {
		mutate(&cfg)
	}
	a, err := NewAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func call(name, args string) tools.FunctionCall {
	return tools.FunctionCall{Name: name, Arguments: args}
}

func TestHITLConfirmationPauseAndContinue(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithConfirmation()}})
	m := &toolCallingModel{planned: []tools.FunctionCall{
		call("Ops_Lookup", `{"to":"bob","subject":"x"}`),
		call("Ops_Send", `{"to":"bob","subject":"hi"}`),
	}}
	a := newHITLAgent(t, m, tool, nil)

	resp, err := a.Run("email bob")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != models.RunStatusPaused || resp.Event != models.RunEventPaused {
		t.Fatalf("expected paused run, got %s/%s", resp.Status, resp.Event)
	}
	if len(resp.Requirements) != 1 {
		t.Fatalf("expected 1 requirement, got %d", len(resp.Requirements))
	}
	req := resp.Requirements[0]
	if !req.NeedsConfirmation() || req.ToolExecution.ToolName != "Ops_Send" || req.ToolExecution.ToolArgs["to"] != "bob" {
		t.Fatalf("bad requirement: %+v", req.ToolExecution)
	}
	if got := tool.called(); len(got) != 1 || got[0] != "lookup:bob" {
		t.Fatalf("only lookup should have run, got %v", got)
	}
	if _, err := a.ContinueRun(context.Background(), &resp); !errors.Is(err, ErrRequirementsUnresolved) {
		t.Fatalf("expected unresolved error, got %v", err)
	}
	if err := req.Confirm(); err != nil {
		t.Fatal(err)
	}
	out, err := a.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != models.RunStatusCompleted || out.RunID != resp.RunID {
		t.Fatalf("expected completed run %s, got %s %s", resp.RunID, out.Status, out.RunID)
	}
	if !strings.Contains(out.TextContent, "found bob") || !strings.Contains(out.TextContent, "sent to bob") {
		t.Fatalf("model should see both tool results: %q", out.TextContent)
	}
	if got := tool.called(); len(got) != 2 || got[1] != "send:bob:hi" {
		t.Fatalf("send should run once after confirmation, got %v", got)
	}
	// The paused state is gone.
	if _, err := a.ContinueRun(context.Background(), resp.RunID); !errors.Is(err, ErrPausedRunNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestHITLRejection(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithConfirmation()}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"to":"eve","subject":"s"}`)}}
	a := newHITLAgent(t, m, tool, nil)
	resp, _ := a.Run("email eve")
	if err := resp.Requirements[0].Reject("not allowed"); err != nil {
		t.Fatal(err)
	}
	out, err := a.ContinueRun(context.Background(), resp.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tool.called()) != 0 {
		t.Fatalf("rejected tool must not run: %v", tool.called())
	}
	if !strings.Contains(out.TextContent, "not allowed") {
		t.Fatalf("model should see the rejection note: %q", out.TextContent)
	}
	if !out.ToolExecutions[0].ToolCallError {
		t.Fatal("rejected call should be marked as error")
	}
}

func TestHITLUserInput(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithUserInput("to")}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"subject":"report"}`)}}
	a := newHITLAgent(t, m, tool, nil)
	resp, err := a.Run("send the report")
	if err != nil {
		t.Fatal(err)
	}
	// "to" is hidden from the model.
	props := m.seenSchemas["Ops_Send"]["properties"].(map[string]interface{})
	if _, ok := props["to"]; ok {
		t.Fatal("user input field must not be in the model schema")
	}
	req := resp.Requirements[0]
	if !req.NeedsUserInput() || len(req.UserInputSchema()) != 1 || req.UserInputSchema()[0].Name != "to" {
		t.Fatalf("bad user input requirement: %+v", req.ToolExecution)
	}
	if err := req.ProvideUserInput(map[string]interface{}{"to": "carol"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ContinueRun(context.Background(), &resp); err != nil {
		t.Fatal(err)
	}
	if got := tool.called(); len(got) != 1 || got[0] != "send:carol:report" {
		t.Fatalf("unexpected calls %v", got)
	}
}

func TestHITLExternalExecution(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithExternalExecution()}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"to":"dan","subject":"s"}`)}}
	a := newHITLAgent(t, m, tool, nil)
	resp, _ := a.Run("send")
	req := resp.Requirements[0]
	if req.PauseType() != models.PauseTypeExternalExecution || !req.NeedsExternalExecution() {
		t.Fatalf("expected external execution requirement")
	}
	if err := req.SetExternalExecutionResult("delivered externally"); err != nil {
		t.Fatal(err)
	}
	out, err := a.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(tool.called()) != 0 || !strings.Contains(out.TextContent, "delivered externally") {
		t.Fatalf("external result should reach the model without running the tool: %q %v", out.TextContent, tool.called())
	}
}

func TestHITLPersistsAcrossRestart(t *testing.T) {
	db := inmemory.New()
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithConfirmation()}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"to":"bob","subject":"s"}`)}}
	cfg := func(c *AgentConfig) { c.DB = db; c.SessionID = "sess-1" }
	a1 := newHITLAgent(t, m, tool, cfg)
	resp, err := a1.Run("email bob")
	if err != nil || !resp.IsPaused() {
		t.Fatalf("expected pause: %v %s", err, resp.Status)
	}

	// Simulate sending the response to a client and back.
	raw, _ := json.Marshal(resp)
	var wire models.RunResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if err := wire.Requirements[0].Confirm(); err != nil {
		t.Fatal(err)
	}

	// A brand new agent (new process) with the same DB and session.
	a2 := newHITLAgent(t, m, tool, cfg)
	stored, err := a2.PausedRun(resp.RunID)
	if err != nil || len(stored.Requirements) != 1 {
		t.Fatalf("paused run should be loadable from the session: %v", err)
	}
	out, err := a2.ContinueRun(context.Background(), resp.RunID, wire.Requirements...)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != models.RunStatusCompleted || len(tool.called()) != 1 {
		t.Fatalf("expected completion: %s %v", out.Status, tool.called())
	}
	session, _ := db.ReadSession(context.Background(), "sess-1")
	if runs, _ := session.SessionData[pausedRunsKey].(map[string]interface{}); len(runs) != 0 {
		t.Fatalf("paused state should be removed after completion: %v", runs)
	}
}

func TestHITLRequiredApproval(t *testing.T) {
	store := approval.NewInMemoryStore()
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithApproval(toolkit.ApprovalRequired)}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"to":"bob","subject":"s"}`)}}
	a := newHITLAgent(t, m, tool, func(c *AgentConfig) { c.ApprovalStore = store })

	resp, _ := a.Run("email bob")
	te := resp.Requirements[0].ToolExecution
	if !te.RequiresConfirmation || te.ApprovalID == "" {
		t.Fatalf("required approval implies confirmation and a record: %+v", te)
	}
	rec, err := store.GetApproval(context.Background(), te.ApprovalID)
	if err != nil || rec.Status != approval.StatusPending || rec.ApprovalType != approval.TypeRequired || rec.RunStatus != models.RunStatusPaused {
		t.Fatalf("bad approval record: %+v %v", rec, err)
	}
	// Confirming the requirement directly is not enough while the approval is pending.
	_ = resp.Requirements[0].Confirm()
	if _, err := a.ContinueRun(context.Background(), &resp); !errors.Is(err, ErrApprovalPending) {
		t.Fatalf("expected pending approval error, got %v", err)
	}
	if _, err := approval.Resolve(context.Background(), store, te.ApprovalID, approval.StatusRejected, "admin", map[string]interface{}{"note": "denied by admin"}); err != nil {
		t.Fatal(err)
	}
	out, err := a.ContinueRun(context.Background(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(tool.called()) != 0 || !strings.Contains(out.TextContent, "rejected") && !strings.Contains(out.TextContent, "denied") {
		t.Fatalf("approval rejection must win: %q %v", out.TextContent, tool.called())
	}
	rec, _ = store.GetApproval(context.Background(), te.ApprovalID)
	if rec.RunStatus != models.RunStatusCompleted {
		t.Fatalf("run status should be updated on completion, got %s", rec.RunStatus)
	}
}

func TestHITLAuditApproval(t *testing.T) {
	store := approval.NewInMemoryStore()
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithConfirmation(), toolkit.WithApproval(toolkit.ApprovalAudit)}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Send", `{"to":"bob","subject":"s"}`)}}
	a := newHITLAgent(t, m, tool, func(c *AgentConfig) { c.ApprovalStore = store })
	resp, _ := a.Run("email bob")
	if recs, _ := store.ListApprovals(context.Background(), approval.Filter{}); len(recs) != 0 {
		t.Fatalf("audit records are written only after resolution")
	}
	_ = resp.Requirements[0].Confirm()
	if _, err := a.ContinueRun(context.Background(), &resp); err != nil {
		t.Fatal(err)
	}
	recs, _ := store.ListApprovals(context.Background(), approval.Filter{RunID: resp.RunID})
	if len(recs) != 1 || recs[0].ApprovalType != approval.TypeAudit || recs[0].Status != approval.StatusApproved {
		t.Fatalf("expected one approved audit record, got %+v", recs)
	}
}

func TestHITLStopAfterToolCall(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Lookup": {toolkit.WithStopAfterCall()}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Lookup", `{"to":"zed","subject":"s"}`)}}
	a := newHITLAgent(t, m, tool, nil)
	out, err := a.Run("lookup zed")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != models.RunStatusCompleted || out.TextContent != "found zed" {
		t.Fatalf("stop_after_tool_call should end with the tool result: %s %q", out.Status, out.TextContent)
	}
	if m.invocations != 2 { // the follow-up model request was aborted
		t.Fatalf("expected the second model request to be cancelled, invocations=%d", m.invocations)
	}
}

func TestHITLShowResult(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Lookup": {toolkit.WithShowResult()}})
	m := &toolCallingModel{planned: []tools.FunctionCall{call("Ops_Lookup", `{"to":"amy","subject":"s"}`)}}
	a := newHITLAgent(t, m, tool, nil)
	out, err := a.Run("lookup amy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.TextContent, "found amy\n\nfinal:") {
		t.Fatalf("show_result should prefix the tool result: %q", out.TextContent)
	}
}

func TestHITLToolOutsideRunIsRefused(t *testing.T) {
	tool := newOpsTool(map[string][]toolkit.MethodOption{"Send": {toolkit.WithConfirmation()}})
	a := newHITLAgent(t, &toolCallingModel{}, tool, nil)
	wrapped := a.GetToolByName("Ops")
	if _, err := wrapped.Execute("Ops_Send", json.RawMessage(`{"to":"x","subject":"y"}`)); err == nil {
		t.Fatal("HITL tool must not run without a pausing run")
	}
	if len(tool.called()) != 0 {
		t.Fatal("tool must not have run")
	}
}

// --- hooks ---

func TestPreRunHooksSemantics(t *testing.T) {
	m := &fakeModel{handler: func(_ context.Context, msgs []models.Message) (*models.MessageResponse, error) {
		return &models.MessageResponse{Content: "echo: " + msgs[len(msgs)-1].Content}, nil
	}}
	var post []string
	a, err := NewAgent(AgentConfig{
		Model: m,
		PreRunHooks: []Hook{
			HookFunc(func(context.Context, *HookArgs) error { return errors.New("non-fatal") }),
			HookFunc(func(_ context.Context, args *HookArgs) error {
				args.Input.Content = strings.ReplaceAll(args.Input.Content, "secret", "[redacted]")
				return nil
			}),
			GuardrailHook(guardrails.NewKeywordGuardrail([]string{"forbidden"})),
		},
		PostRunHooks: []Hook{
			HookFunc(func(_ context.Context, args *HookArgs) error {
				post = append(post, args.RunID)
				args.Output.TextContent += " [checked]"
				return nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Run("my secret plan")
	if err != nil {
		t.Fatalf("non-check hook errors must not abort the run: %v", err)
	}
	if !strings.Contains(out.TextContent, "[redacted]") || strings.Contains(out.TextContent, "secret") {
		t.Fatalf("pre-hook should rewrite the input: %q", out.TextContent)
	}
	if !strings.HasSuffix(out.TextContent, "[checked]") || len(post) != 1 || post[0] != out.RunID {
		t.Fatalf("post-hook should modify the output: %q %v", out.TextContent, post)
	}

	_, err = a.Run("this is forbidden")
	ce, ok := guardrails.AsCheckError(err)
	if !ok || ce.Stage != guardrails.StageInput {
		t.Fatalf("guardrail hook must abort with an input check error, got %v", err)
	}
}

func TestGuardrailPostHookBlocks(t *testing.T) {
	m := &fakeModel{handler: defaultHandler("the password is hunter2")}
	a, err := NewAgent(AgentConfig{
		Model:        m,
		PostRunHooks: GuardrailHooks(guardrails.NewKeywordGuardrail([]string{"password"})),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Run("tell me")
	if ce, ok := guardrails.AsCheckError(err); !ok || ce.Stage != guardrails.StageOutput {
		t.Fatalf("expected output check error, got %v", err)
	}
}

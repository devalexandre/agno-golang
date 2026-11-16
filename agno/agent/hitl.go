package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/storage"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// Human-in-the-loop (HITL) runtime.
//
// Model clients execute tools inside Invoke. Tools carrying HITL flags
// (toolkit.WithConfirmation, WithUserInput, WithExternalExecution) are wrapped
// in a hitlTool: when the model calls one, the call is recorded as a
// requirement, the in-flight model invocation is cancelled and Run returns a
// RunResponse with Status PAUSED. The paused state (messages, tool calls and
// requirements) is kept in memory and, when a DB is configured, in the
// session's session_data["paused_runs"], so ContinueRun works across process
// restarts. WithStopAfterCall / WithShowResult are honoured the same way.

var (
	// errRunPaused is returned to the model client by a tool needing a human.
	errRunPaused = errors.New("agno: run paused waiting for human-in-the-loop resolution")
	// ErrPausedRunNotFound is returned by ContinueRun when no paused state exists for the run.
	ErrPausedRunNotFound = errors.New("paused run not found")
	// ErrRequirementsUnresolved is returned by ContinueRun when requirements are still open.
	ErrRequirementsUnresolved = errors.New("run requirements are not resolved")
	// ErrApprovalPending is returned by ContinueRun when a required approval is still pending.
	ErrApprovalPending = errors.New("approval is still pending; resolve the approval before continuing this run")
	// ErrApprovalNotFound is returned by ContinueRun when a required approval record is missing.
	ErrApprovalNotFound = errors.New("no approval record found for this run; cannot continue a run that requires external approval")
)

const pausedRunsKey = "paused_runs"

// hitlRecorder collects the tool calls of one model invocation.
type hitlRecorder struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	executed []*models.ToolExecution
	paused   []*models.ToolExecution
	shown    []string
	stopped  bool
}

func (r *hitlRecorder) interrupted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.paused) > 0 || r.stopped
}

func (r *hitlRecorder) shownText() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.shown, "\n")
}

// pausedRun is the persisted state of a paused run.
type pausedRun struct {
	RunID        string                   `json:"run_id"`
	SessionID    string                   `json:"session_id"`
	UserID       string                   `json:"user_id,omitempty"`
	Prompt       string                   `json:"prompt"`
	Messages     []models.Message         `json:"messages"`
	Tools        []*models.ToolExecution  `json:"tools"`
	Requirements []*models.RunRequirement `json:"requirements"`
	CreatedAt    int64                    `json:"created_at"`
}

// hitlTool intercepts calls to methods with HITL flags.
type hitlTool struct {
	toolkit.Tool
	agent *Agent
}

// Unwrap returns the wrapped tool.
func (h *hitlTool) Unwrap() toolkit.Tool { return h.Tool }

// Execute pauses on HITL methods and records every other call.
func (h *hitlTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	m, ok := h.Tool.GetMethods()[methodName]
	rec := h.agent.currentHITL()
	if !ok {
		return h.Tool.Execute(methodName, input)
	}
	if m.RequiresHITL() && newToolExecution(methodName, input, m).IsPaused() {
		if rec == nil {
			return nil, fmt.Errorf("tool %s requires human-in-the-loop resolution; call it through Agent.Run, which pauses the run", methodName)
		}
		te := newToolExecution(methodName, input, m)
		rec.mu.Lock()
		rec.paused = append(rec.paused, te)
		rec.mu.Unlock()
		rec.cancel()
		return nil, errRunPaused
	}

	result, err := h.Tool.Execute(methodName, input)
	if rec == nil {
		return result, err
	}
	te := newToolExecution(methodName, input, m)
	te.RequiresConfirmation, te.RequiresUserInput, te.ExternalExecutionRequired = false, false, false
	res := formatToolValue(result)
	if err != nil {
		res = fmt.Sprintf("Error executing tool: %v", err)
		te.ToolCallError = true
	}
	te.Result = &res
	te.StopAfterToolCall = m.StopAfterCall
	rec.mu.Lock()
	rec.executed = append(rec.executed, te)
	if m.ShowResult {
		rec.shown = append(rec.shown, res)
	}
	if m.StopAfterCall {
		rec.stopped = true
	}
	rec.mu.Unlock()
	if m.StopAfterCall {
		rec.cancel()
	}
	return result, err
}

func newToolExecution(methodName string, input json.RawMessage, m toolkit.Method) *models.ToolExecution {
	args := map[string]interface{}{}
	if len(input) > 0 {
		_ = json.Unmarshal(input, &args)
	}
	te := &models.ToolExecution{
		ToolCallID:                "call_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24],
		ToolName:                  methodName,
		ToolArgs:                  args,
		CreatedAt:                 time.Now().Unix(),
		RequiresConfirmation:      m.RequiresConfirmation,
		RequiresUserInput:         m.RequiresUserInput,
		ExternalExecutionRequired: m.ExternalExecution,
		ExternalExecutionSilent:   m.ExternalExecutionSilent,
		ApprovalType:              m.ApprovalType,
	}
	specs := m.UserInputSchema
	if m.UserInputBuilder != nil {
		specs = m.UserInputBuilder(args)
	}
	for _, f := range specs {
		te.UserInputSchema = append(te.UserInputSchema, &models.UserInputField{
			Name: f.Name, FieldType: f.FieldType, Description: f.Description,
		})
	}
	if m.UserFeedbackBuilder != nil {
		for _, q := range m.UserFeedbackBuilder(args) {
			fq := &models.UserFeedbackQuestion{Question: q.Question, Header: q.Header, MultiSelect: q.MultiSelect}
			for _, o := range q.Options {
				fq.Options = append(fq.Options, &models.UserFeedbackOption{Label: o.Label, Description: o.Description})
			}
			te.UserFeedbackSchema = append(te.UserFeedbackSchema, fq)
		}
	}
	// A dynamic pause (get_user_input / ask_user) with nothing to ask runs
	// the tool normally, as in Python.
	if m.IsDynamicUserInput() && len(te.UserInputSchema) == 0 && len(te.UserFeedbackSchema) == 0 {
		te.RequiresUserInput = false
	}
	return te
}

func formatToolValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// toolHasHITLFlags reports whether any method of the tool needs interception.
func toolHasHITLFlags(t toolkit.Tool) bool {
	for _, m := range t.GetMethods() {
		if m.RequiresHITL() || m.StopAfterCall || m.ShowResult {
			return true
		}
	}
	return false
}

func (a *Agent) wrapHITLTool(t toolkit.Tool) toolkit.Tool {
	if t == nil {
		return t
	}
	if _, ok := t.(*hitlTool); ok {
		return t
	}
	if toolHasHITLFlags(t) {
		return &hitlTool{Tool: t, agent: a}
	}
	return t
}

func (a *Agent) setupHITL(config AgentConfig) {
	a.pausedRuns = map[string]*pausedRun{}
	a.preRunHooks = config.PreRunHooks
	a.postRunHooks = config.PostRunHooks
	a.approvalStore = config.ApprovalStore
	if a.approvalStore == nil {
		if s, ok := config.DB.(approval.Store); ok {
			a.approvalStore = s
		}
	}
	for i, t := range a.tools {
		a.tools[i] = a.wrapHITLTool(t)
	}
}

func (a *Agent) currentHITL() *hitlRecorder {
	a.hitlMu.Lock()
	defer a.hitlMu.Unlock()
	return a.hitl
}

// beginHITL starts recording tool calls for a model invocation.
func (a *Agent) beginHITL() *hitlRecorder {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	rec := &hitlRecorder{ctx: ctx, cancel: cancel}
	a.hitlMu.Lock()
	a.hitl = rec
	a.hitlMu.Unlock()
	return rec
}

func (a *Agent) endHITL(rec *hitlRecorder) {
	a.hitlMu.Lock()
	if a.hitl == rec {
		a.hitl = nil
	}
	a.hitlMu.Unlock()
	rec.cancel()
}

// resetForRetry drops what a failed invocation recorded.
func (r *hitlRecorder) resetForRetry() {
	r.mu.Lock()
	r.executed, r.shown = nil, nil
	r.mu.Unlock()
}

// handleInterrupt turns a paused or stopped invocation into a RunResponse.
func (a *Agent) handleInterrupt(rec *hitlRecorder, prompt string, messages []models.Message) (models.RunResponse, error) {
	rec.mu.Lock()
	executed := append([]*models.ToolExecution(nil), rec.executed...)
	paused := append([]*models.ToolExecution(nil), rec.paused...)
	rec.mu.Unlock()

	if len(paused) > 0 {
		return a.pauseRun(prompt, messages, executed, paused)
	}

	// stop_after_tool_call: the run ends with the tool results as content.
	all := append(append([]models.Message(nil), messages...), toolCallMessages(executed)...)
	return a.completeHITLRun(prompt, rec.shownText(), all, a.model.GetID(), executed, nil)
}

// toolCallMessages renders tool executions as an assistant tool-call message
// followed by one tool message per call.
func toolCallMessages(tes []*models.ToolExecution) []models.Message {
	if len(tes) == 0 {
		return nil
	}
	assistant := models.Message{Role: models.TypeAssistantRole}
	var results []models.Message
	for _, te := range tes {
		argsJSON, _ := json.Marshal(te.ToolArgs)
		assistant.ToolCalls = append(assistant.ToolCalls, tools.ToolCall{
			ID:       te.ToolCallID,
			Type:     "function",
			Function: tools.FunctionCall{Name: te.ToolName, Arguments: string(argsJSON)},
		})
		id := te.ToolCallID
		results = append(results, models.Message{Role: models.TypeToolRole, Content: te.ResultString(), ToolCallID: &id})
	}
	return append([]models.Message{assistant}, results...)
}

func (a *Agent) runID() string {
	if a.currentRun != nil {
		return a.currentRun.id
	}
	return ""
}

// pauseRun persists the paused state and builds the PAUSED response.
func (a *Agent) pauseRun(prompt string, messages []models.Message, executed, paused []*models.ToolExecution) (models.RunResponse, error) {
	runID := a.runID()
	reqs := make([]*models.RunRequirement, 0, len(paused))
	for _, te := range paused {
		reqs = append(reqs, models.NewRunRequirement(te))
	}
	a.createRequiredApproval(runID, paused, reqs)

	st := &pausedRun{
		RunID:        runID,
		SessionID:    a.sessionID,
		UserID:       a.userID,
		Prompt:       prompt,
		Messages:     append([]models.Message(nil), messages...),
		Tools:        append(executed, paused...),
		Requirements: reqs,
		CreatedAt:    time.Now().Unix(),
	}
	if err := a.savePausedRun(st); err != nil {
		log.Printf("Warning: failed to persist paused run %s: %v", runID, err)
	}

	return models.RunResponse{
		TextContent:    pausedContent(paused),
		ContentType:    "text",
		Event:          models.RunEventPaused,
		Status:         models.RunStatusPaused,
		RunID:          runID,
		SessionID:      a.sessionID,
		Model:          a.model.GetID(),
		Requirements:   reqs,
		ToolExecutions: st.Tools,
		CreatedAt:      time.Now().Unix(),
	}, nil
}

// pausedContent mirrors Python's paused-run message.
func pausedContent(paused []*models.ToolExecution) string {
	var b strings.Builder
	b.WriteString("I have tools to execute, but I need")
	need := map[models.PauseType]bool{}
	for _, te := range paused {
		if te.ExternalExecutionRequired && te.ExternalExecutionSilent {
			continue
		}
		need[te.PauseType()] = true
	}
	var parts []string
	if need[models.PauseTypeConfirmation] {
		parts = append(parts, "confirmation")
	}
	if need[models.PauseTypeUserInput] || need[models.PauseTypeUserFeedback] {
		parts = append(parts, "user input")
	}
	if need[models.PauseTypeExternalExecution] {
		parts = append(parts, "external execution")
	}
	if len(parts) == 0 {
		return ""
	}
	b.WriteString(" " + strings.Join(parts, " and ") + ".")
	for _, te := range paused {
		args, _ := json.Marshal(te.ToolArgs)
		fmt.Fprintf(&b, "\n- %s(%s)", te.ToolName, args)
	}
	return b.String()
}

// --- approvals ---

func (a *Agent) createRequiredApproval(runID string, paused []*models.ToolExecution, reqs []*models.RunRequirement) {
	if a.approvalStore == nil {
		return
	}
	var pending []*models.ToolExecution
	var names []string
	for _, te := range paused {
		if te.ApprovalType == toolkit.ApprovalRequired {
			pending = append(pending, te)
			names = append(names, te.ToolName)
		}
	}
	if len(pending) == 0 {
		return
	}
	ctxData := map[string]interface{}{"tool_names": names}
	if a.name != "" {
		ctxData["source_name"] = a.name
	}
	rec := &approval.Approval{
		ID:           uuid.New().String(),
		RunID:        runID,
		SessionID:    a.sessionID,
		Status:       approval.StatusPending,
		ApprovalType: approval.TypeRequired,
		PauseType:    pending[0].PauseType(),
		ToolName:     pending[0].ToolName,
		ToolArgs:     pending[0].ToolArgs,
		SourceType:   "agent",
		AgentID:      a.GetID(),
		UserID:       a.userID,
		SourceName:   a.name,
		Requirements: reqs,
		Context:      ctxData,
		RunStatus:    models.RunStatusPaused,
		CreatedAt:    time.Now().Unix(),
	}
	if err := a.approvalStore.CreateApproval(a.ctx, rec); err != nil {
		log.Printf("Warning: error creating approval record: %v", err)
		return
	}
	for _, te := range pending {
		te.ApprovalID = rec.ID
	}
}

func (a *Agent) createAuditApproval(runID string, te *models.ToolExecution, status approval.Status) {
	if a.approvalStore == nil || te.ApprovalType != toolkit.ApprovalAudit {
		return
	}
	now := time.Now().Unix()
	ctxData := map[string]interface{}{"tool_names": []string{te.ToolName}}
	if a.name != "" {
		ctxData["source_name"] = a.name
	}
	rec := &approval.Approval{
		ID:           uuid.New().String(),
		RunID:        runID,
		SessionID:    a.sessionID,
		Status:       status,
		ApprovalType: approval.TypeAudit,
		PauseType:    te.PauseType(),
		ToolName:     te.ToolName,
		ToolArgs:     te.ToolArgs,
		SourceType:   "agent",
		AgentID:      a.GetID(),
		UserID:       a.userID,
		SourceName:   a.name,
		Context:      ctxData,
		ResolvedAt:   now,
		CreatedAt:    now,
	}
	if err := a.approvalStore.CreateApproval(a.ctx, rec); err != nil {
		log.Printf("Warning: error creating audit approval record: %v", err)
	}
}

// applyApprovalGate enforces required approvals and applies their decision.
func (a *Agent) applyApprovalGate(st *pausedRun) error {
	if a.approvalStore == nil {
		return nil
	}
	for _, req := range st.Requirements {
		te := req.ToolExecution
		if te == nil || te.ApprovalType != toolkit.ApprovalRequired || te.Result != nil && !te.ExternalExecutionRequired {
			continue
		}
		if te.ApprovalID == "" {
			return ErrApprovalNotFound
		}
		rec, err := a.approvalStore.GetApproval(a.ctx, te.ApprovalID)
		if err != nil || rec == nil {
			return ErrApprovalNotFound
		}
		switch rec.Status {
		case approval.StatusPending:
			return ErrApprovalPending
		case approval.StatusApproved:
			if te.RequiresConfirmation {
				yes := true
				te.Confirmed, req.Confirmation = &yes, &yes
			}
			if te.RequiresUserInput && rec.ResolutionData != nil {
				values, _ := rec.ResolutionData["values"].(map[string]interface{})
				if values == nil {
					values = rec.ResolutionData
				}
				for _, f := range te.UserInputSchema {
					if v, ok := values[f.Name]; ok {
						f.Value = v
					}
				}
				te.Answered = true
			}
			if te.ExternalExecutionRequired && rec.ResolutionData != nil {
				if r, ok := rec.ResolutionData["result"]; ok {
					s := formatToolValue(r)
					te.Result, req.ExternalExecutionResult = &s, &s
				}
			}
		case approval.StatusRejected:
			no := false
			te.Confirmed, req.Confirmation = &no, &no
			if note, ok := rec.ResolutionData["note"].(string); ok && te.ConfirmationNote == "" {
				te.ConfirmationNote = note
			}
		}
	}
	return nil
}

// --- persistence ---

func (a *Agent) savePausedRun(st *pausedRun) error {
	a.hitlMu.Lock()
	if a.pausedRuns == nil {
		a.pausedRuns = map[string]*pausedRun{}
	}
	a.pausedRuns[st.RunID] = st
	a.hitlMu.Unlock()
	return a.updatePausedRunsInDB(st.SessionID, func(m map[string]interface{}) error {
		raw, err := json.Marshal(st)
		if err != nil {
			return err
		}
		var v map[string]interface{}
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		m[st.RunID] = v
		return nil
	})
}

func (a *Agent) deletePausedRun(st *pausedRun) {
	a.hitlMu.Lock()
	delete(a.pausedRuns, st.RunID)
	a.hitlMu.Unlock()
	if err := a.updatePausedRunsInDB(st.SessionID, func(m map[string]interface{}) error {
		delete(m, st.RunID)
		return nil
	}); err != nil {
		log.Printf("Warning: failed to remove paused run %s: %v", st.RunID, err)
	}
}

func (a *Agent) readSessionForUpdate(sessionID string) (*storage.AgentSession, bool) {
	session, err := a.db.ReadSession(a.ctx, sessionID)
	if err == nil && session != nil {
		return session, true
	}
	now := time.Now().Unix()
	return &storage.AgentSession{
		Session: storage.Session{
			SessionID:   sessionID,
			UserID:      a.userID,
			Memory:      map[string]interface{}{},
			SessionData: map[string]interface{}{},
			ExtraData:   map[string]interface{}{},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		AgentID:   "default-agent",
		AgentData: map[string]interface{}{},
	}, false
}

func (a *Agent) updatePausedRunsInDB(sessionID string, mutate func(map[string]interface{}) error) error {
	if a.db == nil || sessionID == "" {
		return nil
	}
	session, exists := a.readSessionForUpdate(sessionID)
	if session.SessionData == nil {
		session.SessionData = map[string]interface{}{}
	}
	runs := map[string]interface{}{}
	if existing, ok := session.SessionData[pausedRunsKey].(map[string]interface{}); ok {
		for k, v := range existing {
			runs[k] = v
		}
	}
	if err := mutate(runs); err != nil {
		return err
	}
	session.SessionData[pausedRunsKey] = runs
	session.UpdatedAt = time.Now().Unix()
	if exists {
		return a.db.UpdateSession(a.ctx, session)
	}
	if err := a.db.CreateSession(a.ctx, session); err != nil {
		return a.db.UpdateSession(a.ctx, session)
	}
	return nil
}

func (a *Agent) loadPausedRun(runID, sessionID string) (*pausedRun, error) {
	a.hitlMu.Lock()
	st, ok := a.pausedRuns[runID]
	a.hitlMu.Unlock()
	if ok {
		return st, nil
	}
	if a.db == nil {
		return nil, ErrPausedRunNotFound
	}
	if sessionID == "" {
		sessionID = a.sessionID
	}
	session, err := a.db.ReadSession(a.ctx, sessionID)
	if err != nil || session == nil || session.SessionData == nil {
		return nil, ErrPausedRunNotFound
	}
	runs, _ := session.SessionData[pausedRunsKey].(map[string]interface{})
	raw, ok := runs[runID]
	if !ok {
		return nil, ErrPausedRunNotFound
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	st = &pausedRun{}
	if err := json.Unmarshal(b, st); err != nil {
		return nil, fmt.Errorf("decode paused run: %w", err)
	}
	// Requirements and tools must share ToolExecution pointers.
	byID := map[string]*models.ToolExecution{}
	for _, te := range st.Tools {
		byID[te.ToolCallID] = te
	}
	for _, req := range st.Requirements {
		if req.ToolExecution != nil {
			if te, ok := byID[req.ToolExecution.ToolCallID]; ok {
				req.ToolExecution = te
			}
		}
	}
	return st, nil
}

// PausedRun returns the paused response stored for runID (from memory or the
// session DB), so a caller can inspect and resolve its requirements.
func (a *Agent) PausedRun(runID string) (models.RunResponse, error) {
	st, err := a.loadPausedRun(runID, "")
	if err != nil {
		return models.RunResponse{}, err
	}
	var paused []*models.ToolExecution
	for _, r := range st.Requirements {
		paused = append(paused, r.ToolExecution)
	}
	return models.RunResponse{
		TextContent:    pausedContent(paused),
		ContentType:    "text",
		Event:          models.RunEventPaused,
		Status:         models.RunStatusPaused,
		RunID:          st.RunID,
		SessionID:      st.SessionID,
		Requirements:   st.Requirements,
		ToolExecutions: st.Tools,
		CreatedAt:      st.CreatedAt,
	}, nil
}

// --- continue ---

// ContinueRun resumes a PAUSED run once its requirements are resolved.
//
// run is the paused run: its run ID (string), or the paused RunResponse
// (models.RunResponse or *models.RunResponse) whose Requirements were resolved
// with Confirm / Reject / ProvideUserInput / SetExternalExecutionResult.
// Resolved requirements may also be passed explicitly (matched by ID or tool
// call ID), e.g. after the RunResponse was serialized and sent to a client.
//
// Confirmed calls are executed, rejected ones are reported to the model as
// rejected, user input is merged into the call arguments and external results
// are used as the call result; then the model loop resumes. The run may pause
// again. Calls marked with an approval type "required" can only continue
// once their approval record is resolved (see approval.Resolve).
func (a *Agent) ContinueRun(ctx context.Context, run interface{}, requirements ...*models.RunRequirement) (models.RunResponse, error) {
	var runID, sessionID string
	var given []*models.RunRequirement
	switch v := run.(type) {
	case string:
		runID = v
	case *models.RunResponse:
		if v == nil {
			return models.RunResponse{}, errors.New("ContinueRun: nil run response")
		}
		runID, sessionID, given = v.RunID, v.SessionID, v.Requirements
	case models.RunResponse:
		runID, sessionID, given = v.RunID, v.SessionID, v.Requirements
	default:
		return models.RunResponse{}, fmt.Errorf("ContinueRun: unsupported run type %T", run)
	}
	if runID == "" {
		return models.RunResponse{}, errors.New("ContinueRun: run ID is required")
	}
	given = append(given, requirements...)

	if ctx != nil {
		prev := a.ctx
		merged, cancel := context.WithCancel(prev)
		stop := context.AfterFunc(ctx, cancel)
		a.ctx = merged
		defer func() {
			stop()
			cancel()
			a.ctx = prev
		}()
	}

	return a.executeRun(&RunOptions{RunID: &runID}, func() (models.RunResponse, error) {
		return a.continueRun(runID, sessionID, given)
	})
}

// mergeRequirement copies the resolution of src into dst.
func mergeRequirement(dst, src *models.RunRequirement) {
	if dst == src || src == nil {
		return
	}
	if src.Confirmation != nil {
		c := *src.Confirmation
		dst.Confirmation = &c
		dst.ConfirmationNote = src.ConfirmationNote
	}
	if src.ExternalExecutionResult != nil {
		r := *src.ExternalExecutionResult
		dst.ExternalExecutionResult = &r
	}
	ste, dte := src.ToolExecution, dst.ToolExecution
	if ste == nil || dte == nil || ste == dte {
		return
	}
	if ste.Confirmed != nil {
		c := *ste.Confirmed
		dte.Confirmed = &c
	}
	if ste.ConfirmationNote != "" {
		dte.ConfirmationNote = ste.ConfirmationNote
	}
	if ste.Result != nil && dte.ExternalExecutionRequired {
		r := *ste.Result
		dte.Result = &r
		dte.ToolCallError = dte.ToolCallError || ste.ToolCallError
	}
	values := map[string]interface{}{}
	for _, f := range ste.UserInputSchema {
		if f.Value != nil {
			values[f.Name] = f.Value
		}
	}
	for _, f := range dte.UserInputSchema {
		if v, ok := values[f.Name]; ok && f.Value == nil {
			f.Value = v
		}
	}
	selections := map[string][]string{}
	for _, q := range ste.UserFeedbackSchema {
		if q.SelectedOptions != nil {
			selections[q.Question] = q.SelectedOptions
		}
	}
	for _, q := range dte.UserFeedbackSchema {
		if sel, ok := selections[q.Question]; ok && q.SelectedOptions == nil {
			q.SelectedOptions = append([]string{}, sel...)
			for _, opt := range q.Options {
				for _, l := range sel {
					if opt.Label == l {
						opt.Selected = true
					}
				}
			}
		}
	}
	if ste.Answered {
		dte.Answered = true
		if len(dte.UserInputSchema) == 0 && len(dte.UserFeedbackSchema) == 0 {
			for k, v := range ste.ToolArgs {
				if dte.ToolArgs == nil {
					dte.ToolArgs = map[string]interface{}{}
				}
				dte.ToolArgs[k] = v
			}
		}
	}
}

// syncRequirement pushes requirement-level resolution onto the tool execution.
func syncRequirement(req *models.RunRequirement) {
	te := req.ToolExecution
	if te == nil {
		return
	}
	if req.Confirmation != nil && te.Confirmed == nil {
		c := *req.Confirmation
		te.Confirmed = &c
		if !c && te.ConfirmationNote == "" {
			te.ConfirmationNote = req.ConfirmationNote
		}
	}
	if req.ExternalExecutionResult != nil && te.Result == nil {
		r := *req.ExternalExecutionResult
		te.Result = &r
	}
	if te.RequiresUserInput && len(te.UserFeedbackSchema) > 0 && !te.Answered {
		all := true
		for _, q := range te.UserFeedbackSchema {
			if q.SelectedOptions == nil {
				all = false
			}
		}
		te.Answered = all
	} else if te.RequiresUserInput && len(te.UserInputSchema) > 0 && !te.Answered {
		all := true
		for _, f := range te.UserInputSchema {
			if f.Value == nil {
				all = false
			}
		}
		te.Answered = all
	}
}

func isRejected(te *models.ToolExecution) bool {
	return te.Confirmed != nil && !*te.Confirmed
}

func (a *Agent) continueRun(runID, sessionID string, given []*models.RunRequirement) (models.RunResponse, error) {
	st, err := a.loadPausedRun(runID, sessionID)
	if err != nil {
		return models.RunResponse{}, fmt.Errorf("continue run %s: %w", runID, err)
	}
	if a.currentRun != nil {
		a.currentRun.prompt = st.Prompt
	}

	// Apply the caller's resolutions.
	for _, g := range given {
		for _, req := range st.Requirements {
			if g.ID == req.ID || (g.ToolExecution != nil && req.ToolExecution != nil && g.ToolExecution.ToolCallID == req.ToolExecution.ToolCallID) {
				mergeRequirement(req, g)
			}
		}
	}
	for _, req := range st.Requirements {
		syncRequirement(req)
	}
	if err := a.applyApprovalGate(st); err != nil {
		return models.RunResponse{}, err
	}
	var open []string
	for _, req := range st.Requirements {
		if req.ToolExecution != nil && isRejected(req.ToolExecution) {
			continue
		}
		if !req.IsResolved() {
			open = append(open, fmt.Sprintf("%s (%s)", req.ToolExecution.ToolName, req.PauseType()))
		}
	}
	if len(open) > 0 {
		return models.RunResponse{}, fmt.Errorf("%w: %s", ErrRequirementsUnresolved, strings.Join(open, ", "))
	}

	// Resolve the paused calls.
	for _, req := range st.Requirements {
		a.resolvePausedCall(runID, req.ToolExecution)
	}

	messages := append(append([]models.Message(nil), st.Messages...), toolCallMessages(st.Tools)...)
	return a.resumeModelLoop(st, messages)
}

// resolvePausedCall executes, rejects or fills in the result of a paused call.
func (a *Agent) resolvePausedCall(runID string, te *models.ToolExecution) {
	if te == nil {
		return
	}
	status := approval.StatusApproved
	switch {
	case isRejected(te):
		note := te.ConfirmationNote
		if note == "" {
			note = "Function call was rejected by the user"
		}
		te.ConfirmationNote = note
		te.Result = &note
		te.ToolCallError = true
		status = approval.StatusRejected
	case te.ExternalExecutionRequired:
		// Result supplied by the caller.
	case len(te.UserFeedbackSchema) > 0:
		// ask_user: the model receives the selections (Python handle_ask_user_tool_update).
		res := userFeedbackResult(te)
		te.Result = &res
	case te.RequiresUserInput && a.isDynamicUserInput(te.ToolName):
		// get_user_input: the model receives the values (Python handle_get_user_input_tool_update).
		res := userInputResult(te)
		te.Result = &res
	default:
		if te.RequiresUserInput {
			if te.ToolArgs == nil {
				te.ToolArgs = map[string]interface{}{}
			}
			for _, f := range te.UserInputSchema {
				te.ToolArgs[f.Name] = f.Value
			}
		}
		res := a.executeResolvedCall(te)
		te.Result = &res
	}
	a.createAuditApproval(runID, te, status)
	te.RequiresConfirmation, te.RequiresUserInput, te.ExternalExecutionRequired = false, false, false
	te.Answered = te.Answered || !isRejected(te)
}

// userInputResult renders collected user input for the model.
func userInputResult(te *models.ToolExecution) string {
	type kv struct {
		Name  string      `json:"name"`
		Value interface{} `json:"value"`
	}
	out := make([]kv, 0, len(te.UserInputSchema))
	for _, f := range te.UserInputSchema {
		out = append(out, kv{f.Name, f.Value})
	}
	b, _ := json.Marshal(out)
	return "User inputs retrieved: " + string(b)
}

// userFeedbackResult renders the user's selections for the model.
func userFeedbackResult(te *models.ToolExecution) string {
	type qa struct {
		Question string   `json:"question"`
		Selected []string `json:"selected"`
	}
	out := make([]qa, 0, len(te.UserFeedbackSchema))
	for _, q := range te.UserFeedbackSchema {
		sel := q.SelectedOptions
		if sel == nil {
			sel = []string{}
		}
		out = append(out, qa{q.Question, sel})
	}
	b, _ := json.Marshal(out)
	return "User feedback received: " + string(b)
}

// isDynamicUserInput reports whether the named method builds its user input
// from the call arguments (toolkit.WithUserInputFromArgs / WithUserFeedback).
func (a *Agent) isDynamicUserInput(name string) bool {
	for _, t := range a.tools {
		if m, ok := t.GetMethods()[name]; ok {
			return m.IsDynamicUserInput()
		}
	}
	return false
}

// executeResolvedCall runs a confirmed / answered call, bypassing the HITL wrapper.
func (a *Agent) executeResolvedCall(te *models.ToolExecution) string {
	for _, t := range a.tools {
		inner := t
		if h, ok := t.(*hitlTool); ok {
			inner = h.Tool
		}
		if _, ok := inner.GetMethods()[te.ToolName]; !ok {
			continue
		}
		args, _ := json.Marshal(te.ToolArgs)
		result, err := inner.Execute(te.ToolName, args)
		if err != nil {
			te.ToolCallError = true
			return fmt.Sprintf("Error executing tool: %v", err)
		}
		return formatToolValue(result)
	}
	te.ToolCallError = true
	return fmt.Sprintf("Tool %s not found", te.ToolName)
}

// resumeModelLoop calls the model again after the paused calls were resolved.
func (a *Agent) resumeModelLoop(st *pausedRun, messages []models.Message) (models.RunResponse, error) {
	if err := a.checkCancelled(); err != nil {
		return models.RunResponse{}, err
	}
	rec := a.beginHITL()
	defer a.endHITL(rec)

	opts := append([]models.Option{models.WithTools(a.tools)}, a.modelOptions...)
	resp, err := a.model.Invoke(rec.ctx, messages, opts...)
	if rec.interrupted() {
		return a.handleInterrupt(rec, st.Prompt, messages)
	}
	if err != nil {
		return models.RunResponse{}, err
	}
	// Model clients that return raw tool calls (without executing them).
	if len(resp.ToolCalls) > 0 && len(resp.ToolResults) == 0 {
		finalResult, toolMessages, _, _, perr := a.processToolCallsFromResponse(resp)
		if rec.interrupted() {
			return a.handleInterrupt(rec, st.Prompt, messages)
		}
		if perr != nil {
			return models.RunResponse{}, fmt.Errorf("tool call processing failed: %w", perr)
		}
		resp.Content = finalResult
		messages = append(messages, toolMessages...)
	}

	content := resp.Content
	if shown := rec.shownText(); shown != "" {
		content = shown + "\n\n" + content
	}
	a.deletePausedRun(st)
	out, err := a.completeHITLRun(st.Prompt, content, messages, resp.Model, st.Tools, st.Requirements)
	if err == nil && a.approvalStore != nil {
		if _, uerr := a.approvalStore.UpdateRunStatus(a.ctx, st.RunID, models.RunStatusCompleted); uerr != nil {
			log.Printf("Warning: error updating approval run status: %v", uerr)
		}
	}
	return out, err
}

// completeHITLRun finishes a run that went through the HITL machinery:
// storage, memory, history, output formatting, output guardrails and post-hooks.
func (a *Agent) completeHITLRun(prompt, content string, messages []models.Message, model string, tes []*models.ToolExecution, reqs []*models.RunRequirement) (models.RunResponse, error) {
	if a.db != nil {
		if err := a.saveRun(prompt, content, messages); err != nil && a.debug {
			fmt.Printf("Warning: Failed to save run: %v\n", err)
		}
	}
	if a.memory != nil {
		if err := a.processMemories(prompt, content); err != nil && a.debug {
			fmt.Printf("Warning: Failed to process memories: %v\n", err)
		}
	}
	if a.addHistoryToMessages {
		a.messages = append(a.messages,
			models.Message{Role: models.TypeUserRole, Content: prompt},
			models.Message{Role: models.TypeAssistantRole, Content: content},
		)
		a.trimHistory()
	}

	parsed, err := a.ApplyOutputFormatting(content)
	if err != nil {
		return models.RunResponse{}, err
	}
	var output interface{}
	if parsed != content {
		output = parsed
	}
	resp := models.RunResponse{
		TextContent:    content,
		ContentType:    "text",
		Event:          "RunResponse",
		ParsedOutput:   parsed,
		Output:         output,
		Messages:       []models.Message{{Role: models.TypeAssistantRole, Content: content}},
		Model:          model,
		Requirements:   reqs,
		ToolExecutions: tes,
		CreatedAt:      time.Now().Unix(),
	}
	if len(a.outputGuardrails) > 0 {
		if err := RunGuardrails(a.ctx, a.outputGuardrails, resp); err != nil {
			return models.RunResponse{}, fmt.Errorf("output validation failed: %w", err)
		}
	}
	for i, hook := range a.postHooks {
		if err := hook(a.ctx, &resp); err != nil {
			return models.RunResponse{}, fmt.Errorf("post-hook %d failed: %w", i, err)
		}
	}
	if err := a.runPostRunHooks(&resp); err != nil {
		return models.RunResponse{}, err
	}
	return resp, nil
}

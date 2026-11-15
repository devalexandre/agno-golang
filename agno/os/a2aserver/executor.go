package a2aserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/google/uuid"
)

// Artifact names (Python RESPONSE_ARTIFACT_NAME / REASONING_ARTIFACT_NAME).
const (
	ResponseArtifact  = "response"
	ReasoningArtifact = "reasoning"
)

type emitFunc func(*a2a.StreamResponse) error

func (s *Server) send(ctx context.Context, c *call, raw json.RawMessage, emit emitFunc) (*a2a.StreamResponse, error) {
	var p a2a.SendMessageParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, rpcErr(a2a.CodeInvalidParams, "Invalid params: %v", err)
	}
	msg := p.Message
	if len(msg.Parts) == 0 {
		return nil, rpcErr(a2a.CodeInvalidParams, "A message with at least one part is required")
	}
	if msg.MessageID == "" {
		msg.MessageID = uuid.NewString()
	}
	msg.Role = a2a.RoleUser
	if c.authed == "" {
		if uid, ok := msg.Metadata["userId"].(string); ok && uid != "" {
			c.userID = uid
		}
	}
	if msg.TaskID != "" {
		return s.continueTask(ctx, c, msg, emit)
	}
	input, err := messageInput(&msg)
	if err != nil {
		return nil, rpcErr(a2a.CodeInvalidParams, "%v", err)
	}
	contextID := msg.ContextID
	if contextID == "" {
		contextID = uuid.NewString()
	}
	msg.ContextID = contextID
	taskID := uuid.NewString()
	msg.TaskID = taskID
	rec := &TaskRecord{
		Task:        a2a.Task{ID: taskID, ContextID: contextID, Status: a2a.NewStatus(a2a.StateSubmitted, nil), History: []a2a.Message{msg}},
		RunID:       taskID,
		UserID:      c.userID,
		ComponentID: c.comp.ID(),
		Kind:        c.comp.Kind(),
		UpdatedAt:   time.Now().Unix(),
	}
	if err := s.store.Save(ctx, rec); err != nil {
		return nil, err
	}
	if emit != nil {
		task := rec.Task
		if err := emit(&a2a.StreamResponse{Task: &task}); err != nil {
			return nil, err
		}
	}
	blocking := true
	if cfg := p.Configuration; cfg != nil && (cfg.ReturnImmediately || (cfg.Blocking != nil && !*cfg.Blocking)) {
		blocking = false
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.running.Store(taskID, cancel)
	run := func(emit emitFunc) *TaskRecord {
		defer func() {
			cancel()
			s.running.Delete(taskID)
		}()
		return s.execute(runCtx, c, taskID, input, emit)
	}
	if emit == nil && !blocking {
		snapshot, _ := s.update(ctx, taskID, func(r *TaskRecord) bool {
			r.Task.Status = a2a.NewStatus(a2a.StateWorking, nil)
			return true
		})
		go run(nil)
		return &a2a.StreamResponse{Task: &snapshot.Task}, nil
	}
	final := run(emit)
	if final == nil {
		return nil, rpcErr(a2a.CodeInternalError, "task %s was lost", taskID)
	}
	if p.Configuration != nil {
		trimHistory(&final.Task, p.Configuration.HistoryLength)
	}
	return &a2a.StreamResponse{Task: &final.Task}, nil
}

// messageInput maps an A2A message to the run input (Python
// map_a2a_message_to_run_input): text parts are the prompt; data parts are
// appended as JSON; files are referenced by name/URL (the component runtime
// has no media input yet).
func messageInput(m *a2a.Message) (string, error) {
	var parts []string
	for _, p := range m.Parts {
		switch {
		case p.Text != nil:
			parts = append(parts, *p.Text)
		case p.Data != nil:
			b, err := json.Marshal(p.Data)
			if err != nil {
				return "", err
			}
			parts = append(parts, string(b))
		case p.URL != "":
			parts = append(parts, fmt.Sprintf("[file: %s %s]", p.Filename, p.URL))
		case len(p.Raw) > 0:
			parts = append(parts, fmt.Sprintf("[file: %s (%s, %d bytes)]", p.Filename, p.MediaType, len(p.Raw)))
		}
	}
	input := strings.TrimSpace(strings.Join(parts, "\n"))
	if input == "" {
		return "", errors.New("the message carries no content")
	}
	return input, nil
}

// safeEmit drops further events once the client is gone, so the run still
// completes and stays readable with GetTask.
func safeEmit(emit emitFunc) emitFunc {
	if emit == nil {
		return func(*a2a.StreamResponse) error { return nil }
	}
	var mu sync.Mutex
	gone := false
	return func(r *a2a.StreamResponse) error {
		mu.Lock()
		defer mu.Unlock()
		if gone {
			return nil
		}
		if err := emit(r); err != nil {
			gone = true
		}
		return nil
	}
}

func (s *Server) status(taskID, contextID string, st a2a.TaskStatus, meta map[string]any, final bool) *a2a.StreamResponse {
	return &a2a.StreamResponse{StatusUpdate: &a2a.TaskStatusUpdateEvent{TaskID: taskID, ContextID: contextID, Status: st, Metadata: meta, Final: final}}
}

// execute runs the component for a task and returns the final record.
func (s *Server) execute(ctx context.Context, c *call, taskID, input string, emit emitFunc) *TaskRecord {
	emit = safeEmit(emit)
	rec, err := s.update(ctx, taskID, func(r *TaskRecord) bool {
		r.Task.Status = a2a.NewStatus(a2a.StateWorking, nil)
		return true
	})
	if err != nil {
		return nil
	}
	contextID := rec.Task.ContextID
	_ = emit(s.status(taskID, contextID, rec.Task.Status, nil, false))

	responseID, reasoningID := taskID+"-"+ResponseArtifact, taskID+"-"+ReasoningArtifact
	var content, reasoning strings.Builder
	contentStarted, reasoningStarted := false, false
	res, runErr := component.Stream(ctx, c.comp, component.RunRequest{Input: input, SessionID: contextID, UserID: c.userID, RunID: taskID},
		func(ev component.Event) error {
			switch ev.Type {
			case component.EventRunContent:
				if ev.Content == "" {
					return nil
				}
				content.WriteString(ev.Content)
				_ = emit(&a2a.StreamResponse{ArtifactUpdate: &a2a.TaskArtifactUpdateEvent{TaskID: taskID, ContextID: contextID, Append: contentStarted,
					Artifact: a2a.Artifact{ArtifactID: responseID, Name: ResponseArtifact, Parts: []a2a.Part{a2a.TextPart(ev.Content)}, Metadata: map[string]any{"agno_content_category": "content"}}}})
				contentStarted = true
			case component.EventReasoningContent:
				reasoning.WriteString(ev.Content)
				_ = emit(&a2a.StreamResponse{ArtifactUpdate: &a2a.TaskArtifactUpdateEvent{TaskID: taskID, ContextID: contextID, Append: reasoningStarted,
					Artifact: a2a.Artifact{ArtifactID: reasoningID, Name: ReasoningArtifact, Parts: []a2a.Part{a2a.TextPart(ev.Content)},
						Metadata: map[string]any{"agno_content_category": "reasoning", "agno_event_type": "reasoning_content_delta"}}}})
				reasoningStarted = true
			case component.EventToolCallStarted, component.EventToolCallCompleted:
				meta := map[string]any{"agno_event_type": "tool_call_started"}
				if ev.Type == component.EventToolCallCompleted {
					meta["agno_event_type"] = "tool_call_completed"
				}
				if ev.Tool != nil {
					meta["tool_name"] = ev.Tool.Name
					if ev.Tool.ID != "" {
						meta["tool_call_id"] = ev.Tool.ID
					}
					if len(ev.Tool.Args) > 0 {
						b, _ := json.Marshal(ev.Tool.Args)
						meta["tool_args"] = string(b)
					}
				}
				_ = emit(s.status(taskID, contextID, a2a.NewStatus(a2a.StateWorking, nil), meta, false))
			}
			return nil
		})
	if res != nil && res.Content == "" {
		res.Content = content.String()
	}
	return s.finalize(ctx, c, taskID, res, runErr, reasoning.String(), emit)
}

func (s *Server) pausedType(c *call) string {
	if c.comp.Kind() == component.KindWorkflow {
		return "workflow_paused"
	}
	return "run_paused"
}

// PausedParts builds the input-required message parts (Python map_paused_run_to_parts).
func PausedParts(res *component.RunResult) []a2a.Part {
	var pending []string
	for _, r := range res.Requirements {
		if te, ok := r["tool_execution"].(map[string]any); ok {
			if n, _ := te["tool_name"].(string); n != "" {
				pending = append(pending, "tool "+n)
			}
		} else if n, _ := r["tool_name"].(string); n != "" {
			pending = append(pending, "tool "+n)
		}
		if n, _ := r["step_name"].(string); n != "" {
			pending = append(pending, "step "+n)
		}
	}
	note := "Run is paused and requires input to continue"
	if len(pending) > 0 {
		note += ": " + strings.Join(pending, ", ")
	}
	parts := []a2a.Part{a2a.TextPart(note)}
	if len(res.Requirements) > 0 {
		key := "requirements"
		if res.Kind == component.KindWorkflow {
			key = "step_requirements"
		}
		parts = append(parts, a2a.DataPart(map[string]any{key: res.Requirements}))
	}
	return parts
}

func (s *Server) finalize(ctx context.Context, c *call, taskID string, res *component.RunResult, runErr error, reasoning string, emit emitFunc) *TaskRecord {
	var events []*a2a.StreamResponse
	rec, err := s.update(ctx, taskID, func(r *TaskRecord) bool {
		if r.Task.Status.State.IsTerminal() { // cancelled meanwhile
			return false
		}
		if res != nil && res.RunID != "" {
			r.RunID = res.RunID
		}
		var meta map[string]any
		switch {
		case runErr != nil && errors.Is(runErr, context.Canceled):
			r.Task.Status = a2a.NewStatus(a2a.StateCanceled, agentMessage(r, a2a.TextPart("Run was cancelled")))
			meta = map[string]any{"agno_event_type": "run_cancelled"}
		case runErr != nil:
			r.Task.Status = a2a.NewStatus(a2a.StateFailed, agentMessage(r, a2a.TextPart("Error: "+runErr.Error())))
			meta = map[string]any{"agno_event_type": "run_error"}
		case res.Status == component.StatusPaused:
			r.Task.Status = a2a.NewStatus(a2a.StateInputRequired, agentMessage(r, PausedParts(res)...))
			meta = map[string]any{"agno_event_type": s.pausedType(c)}
		case res.Status == component.StatusCancelled:
			r.Task.Status = a2a.NewStatus(a2a.StateCanceled, agentMessage(r, a2a.TextPart("Run was cancelled")))
			meta = map[string]any{"agno_event_type": "run_cancelled"}
		case res.Status == component.StatusError:
			msg := res.Error
			if msg == "" {
				msg = res.Content
			}
			if msg == "" {
				msg = "Run failed"
			}
			r.Task.Status = a2a.NewStatus(a2a.StateFailed, agentMessage(r, a2a.TextPart(msg)))
			meta = map[string]any{"agno_event_type": "run_error"}
		default:
			r.Task.Status = a2a.NewStatus(a2a.StateCompleted, nil)
			meta = map[string]any{}
			for k, v := range res.Metadata {
				meta[k] = v
			}
			if len(res.Metrics) > 0 {
				meta["metrics"] = res.Metrics
			}
			if r.UserID != "" {
				meta["userId"] = r.UserID
			}
		}
		if res != nil && res.Content != "" && runErr == nil && res.Status != component.StatusError {
			art := a2a.Artifact{ArtifactID: taskID + "-" + ResponseArtifact, Name: ResponseArtifact,
				Parts: []a2a.Part{contentPart(res)}, Metadata: map[string]any{"agno_content_category": "content"}}
			r.Task.Artifacts = upsertArtifact(r.Task.Artifacts, art)
			events = append(events, &a2a.StreamResponse{ArtifactUpdate: &a2a.TaskArtifactUpdateEvent{TaskID: taskID, ContextID: r.Task.ContextID, Artifact: art, LastChunk: true}})
		}
		if reasoning != "" {
			r.Task.Artifacts = upsertArtifact(r.Task.Artifacts, a2a.Artifact{ArtifactID: taskID + "-" + ReasoningArtifact, Name: ReasoningArtifact,
				Parts: []a2a.Part{a2a.TextPart(reasoning)}, Metadata: map[string]any{"agno_content_category": "reasoning"}})
		}
		if r.Task.Status.Message != nil {
			r.Task.History = append(r.Task.History, *r.Task.Status.Message)
		}
		if len(meta) == 0 {
			meta = nil
		}
		final := r.Task.Status.State.IsTerminal() || r.Task.Status.State == a2a.StateInputRequired
		events = append(events, s.status(taskID, r.Task.ContextID, r.Task.Status, meta, final))
		r.UpdatedAt = time.Now().Unix()
		return true
	})
	if err != nil {
		return nil
	}
	for _, ev := range events {
		_ = emit(ev)
	}
	if len(events) == 0 { // cancelled meanwhile: report the stored state
		_ = emit(s.status(taskID, rec.Task.ContextID, rec.Task.Status, nil, true))
	}
	return rec
}

// contentPart is a text part, or a data part for structured output.
func contentPart(res *component.RunResult) a2a.Part {
	switch res.Output.(type) {
	case map[string]any, []any:
		return a2a.DataPart(res.Output)
	}
	return a2a.TextPart(res.Content)
}

func upsertArtifact(list []a2a.Artifact, a a2a.Artifact) []a2a.Artifact {
	for i := range list {
		if list[i].ArtifactID == a.ArtifactID {
			list[i] = a
			return list
		}
	}
	return append(list, a)
}

// requirementIDs collects the ids of the requirements in data payloads: a
// tool requirement is known by its tool call id (Python _get_requirement_ids).
func requirementIDs(payloads []any) map[string]bool {
	ids := map[string]bool{}
	for _, d := range payloads {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"requirements", "step_requirements"} {
			list, _ := m[key].([]any)
			for _, item := range list {
				r, ok := item.(map[string]any)
				if !ok {
					continue
				}
				id := ""
				if te, ok := r["tool_execution"].(map[string]any); ok {
					id, _ = te["tool_call_id"].(string)
				}
				for _, k := range []string{"tool_call_id", "id", "step_id"} {
					if id == "" {
						id = fmt.Sprint(valueOr(r[k], ""))
					}
				}
				ids[id] = true
			}
		}
	}
	return ids
}

func valueOr(v any, d any) any {
	if v == nil {
		return d
	}
	return v
}

// continueTask continues the paused run of a task with the requirements the
// client resolved (Python A2AExecutor._continue_task).
func (s *Server) continueTask(ctx context.Context, c *call, msg a2a.Message, emit emitFunc) (*a2a.StreamResponse, error) {
	rec, err := s.load(ctx, c, msg.TaskID)
	if err != nil {
		return nil, err
	}
	if rec.Task.Status.State.IsTerminal() {
		return nil, rpcErr(a2a.CodeUnsupportedOperation, "Task %s is in a terminal state (%s) and cannot accept messages", rec.Task.ID, rec.Task.Status.State.V03())
	}
	emit = safeEmit(emit)
	if rec.UserID != "" && c.authed == "" {
		c.userID = rec.UserID // the run is continued as its owner
	}
	if rec.Task.Status.State != a2a.StateInputRequired {
		task := rec.Task
		_ = emit(&a2a.StreamResponse{Task: &task})
		return &a2a.StreamResponse{Task: &task}, nil
	}
	var requirements []map[string]any
	var sent []any
	for _, d := range msg.DataParts() {
		sent = append(sent, d)
		if m, ok := d.(map[string]any); ok {
			for _, key := range []string{"requirements", "step_requirements"} {
				if list, ok := m[key].([]any); ok {
					for _, item := range list {
						if r, ok := item.(map[string]any); ok {
							requirements = append(requirements, r)
						}
					}
				}
			}
		}
	}
	var pending []any
	var pendingParts []a2a.Part
	if m := rec.Task.Status.Message; m != nil {
		pending = m.DataParts()
		for _, p := range m.Parts {
			if p.Data != nil {
				pendingParts = append(pendingParts, p)
			}
		}
	}
	waitAgain := func(note string) (*a2a.StreamResponse, error) {
		out, err := s.update(ctx, rec.Task.ID, func(r *TaskRecord) bool {
			r.Task.History = append(r.Task.History, msg)
			if note != "" {
				r.Task.Status = a2a.NewStatus(a2a.StateInputRequired, agentMessage(r, append([]a2a.Part{a2a.TextPart(note)}, pendingParts...)...))
			}
			return true
		})
		if err != nil {
			return nil, err
		}
		_ = emit(s.status(out.Task.ID, out.Task.ContextID, out.Task.Status, map[string]any{"agno_event_type": s.pausedType(c)}, true))
		return &a2a.StreamResponse{Task: &out.Task}, nil
	}
	// Without resolved requirements there is nothing to continue with.
	if len(requirements) == 0 {
		return waitAgain("")
	}
	pendingIDs := requirementIDs(pending)
	for id := range requirementIDs(sent) {
		if !pendingIDs[id] {
			return waitAgain("Error: the requirements sent are not the ones this task waits on")
		}
	}
	if _, err := s.update(ctx, rec.Task.ID, func(r *TaskRecord) bool {
		r.Task.History = append(r.Task.History, msg)
		r.Task.Status = a2a.NewStatus(a2a.StateWorking, nil)
		return true
	}); err != nil {
		return nil, err
	}
	_ = emit(s.status(rec.Task.ID, rec.Task.ContextID, a2a.NewStatus(a2a.StateWorking, nil), nil, false))
	res, runErr := component.Continue(context.WithoutCancel(ctx), c.comp, component.ContinueRequest{
		RunID: rec.RunID, SessionID: rec.Task.ContextID, UserID: c.userID, Requirements: requirements,
	})
	if runErr != nil {
		// A refused continue leaves the run paused: the task keeps waiting.
		if _, err := s.update(ctx, rec.Task.ID, func(r *TaskRecord) bool {
			r.Task.Status = a2a.NewStatus(a2a.StateInputRequired, agentMessage(r, append([]a2a.Part{a2a.TextPart("Error: " + runErr.Error())}, pendingParts...)...))
			return true
		}); err != nil {
			return nil, err
		}
		out, _ := s.store.Get(ctx, rec.Task.ID)
		_ = emit(s.status(out.Task.ID, out.Task.ContextID, out.Task.Status, map[string]any{"agno_event_type": s.pausedType(c)}, true))
		return &a2a.StreamResponse{Task: &out.Task}, nil
	}
	if res.Kind == "" {
		res.Kind = c.comp.Kind()
	}
	if res.Content != "" {
		_ = emit(&a2a.StreamResponse{ArtifactUpdate: &a2a.TaskArtifactUpdateEvent{TaskID: rec.Task.ID, ContextID: rec.Task.ContextID,
			Artifact: a2a.Artifact{ArtifactID: rec.Task.ID + "-" + ResponseArtifact, Name: ResponseArtifact, Parts: []a2a.Part{a2a.TextPart(res.Content)}}}})
	}
	final := s.finalize(ctx, c, rec.Task.ID, res, nil, "", emit)
	if final == nil {
		return nil, rpcErr(a2a.CodeInternalError, "task %s was lost", rec.Task.ID)
	}
	return &a2a.StreamResponse{Task: &final.Task}, nil
}

// ---------------------------------------------------------------- streaming

func (s *Server) stream(w http.ResponseWriter, r *http.Request, c *call, raw json.RawMessage) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.reply(w, c, nil, rpcErr(a2a.CodeInternalError, "streaming unsupported"))
		return
	}
	started := false
	start := func() {
		if started {
			return
		}
		started = true
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
	}
	write := func(payload map[string]any) error {
		if err := r.Context().Err(); err != nil {
			return err
		}
		start()
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	_, err := s.send(r.Context(), c, raw, func(ev *a2a.StreamResponse) error {
		return write(map[string]any{"jsonrpc": "2.0", "id": c.id, "result": c.encode(ev)})
	})
	if err != nil {
		var re *a2a.RPCError
		if !errors.As(err, &re) {
			re = rpcErr(a2a.CodeInternalError, "%v", err)
		}
		if !started {
			s.reply(w, c, nil, re)
			return
		}
		_ = write(map[string]any{"jsonrpc": "2.0", "id": c.id, "error": re})
	}
}

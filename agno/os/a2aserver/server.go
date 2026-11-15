// Package a2aserver exposes Agno components over the A2A protocol (v1.0
// JSON-RPC binding, with v0.3 compatibility). Port of Python
// agno/os/interfaces/a2a.
//
// Routes (under Config.Prefix, default "/a2a"):
//
//	GET  /{agents|teams|workflows}/{id}/.well-known/agent-card.json
//	POST /{agents|teams|workflows}/{id}          JSON-RPC endpoint
//	GET  /.well-known/agent-card.json            (only with a single component)
//
// Methods: SendMessage / message/send, SendStreamingMessage / message/stream
// (SSE), GetTask / tasks/get, CancelTask / tasks/cancel, ListTasks.
//
// A run that pauses (component.StatusPaused, i.e. models.RunStatusPaused)
// leaves the task in TASK_STATE_INPUT_REQUIRED with a data part
// {"requirements": [...]}. The client answers by sending a message with the
// same taskId and a data part carrying the resolved requirements, which
// continues the run (component.Continuer -> Agent.ContinueRun).
package a2aserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/google/uuid"
)

// Config configures the A2A interface.
type Config struct {
	// Prefix is the mount prefix (default "/a2a"; use "/" for none).
	Prefix string
	// BaseURL is the public base URL for agent cards (default: derived from the request).
	BaseURL string
	// SecuritySchemes are advertised on the cards (see a2a.BearerSecuritySchemes).
	SecuritySchemes map[string]map[string]any
	// DisableV03Compat stops advertising/serving v0.3.
	DisableV03Compat bool
	Provider         *a2a.AgentProvider
	DocumentationURL string
	IconURL          string
	// Version is the card version (default "1.0.0").
	Version string
	// TaskStore persists tasks (default: in memory).
	TaskStore TaskStore
	// Identity returns the authenticated caller ("" when anonymous). An
	// anonymous caller may attribute runs with X-User-ID or message
	// metadata.userId; an authenticated one cannot.
	Identity func(r *http.Request) string
	// Authorize gates every card and JSON-RPC request; an error is a 403.
	Authorize func(r *http.Request, kind component.Kind, componentID string) error
}

// Server is an A2A server over a component registry.
type Server struct {
	cfg     Config
	reg     *component.Registry
	store   TaskStore
	locks   sync.Map // task id -> *sync.Mutex
	running sync.Map // task id -> context.CancelFunc
}

// New returns an A2A server.
func New(reg *component.Registry, cfg *Config) (*Server, error) {
	if reg == nil || len(reg.All()) == 0 {
		return nil, errors.New("a2a: agents, teams, or workflows are required to setup the A2A interface")
	}
	c := Config{}
	if cfg != nil {
		c = *cfg
	}
	if c.Prefix == "" {
		c.Prefix = "/a2a"
	}
	c.Prefix = "/" + strings.Trim(c.Prefix, "/")
	if c.Prefix == "/" {
		c.Prefix = ""
	}
	if c.Version == "" {
		c.Version = "1.0.0"
	}
	store := c.TaskStore
	if store == nil {
		store = NewMemoryTaskStore()
	}
	return &Server{cfg: c, reg: reg, store: store}, nil
}

// Prefix returns the mount prefix ("" for root).
func (s *Server) Prefix() string { return s.cfg.Prefix }

// Store returns the task store.
func (s *Server) Store() TaskStore { return s.store }

// Handler returns the HTTP handler. It expects full request paths (prefix included).
func (s *Server) Handler() http.Handler { return s }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(strings.TrimPrefix(r.URL.Path, s.cfg.Prefix), "/")
	const card = "/.well-known/agent-card.json"
	if path == card {
		all := s.reg.All()
		if len(all) != 1 {
			writeJSON(w, http.StatusNotFound, map[string]any{"detail": "multiple components: use /{agents|teams|workflows}/{id}" + card})
			return
		}
		s.serveCard(w, r, all[0])
		return
	}
	isCard := strings.HasSuffix(path, card)
	path = strings.TrimSuffix(path, card)
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 2 {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found"})
		return
	}
	kind, ok := component.ParseKind(parts[0])
	if !ok || parts[0] != kind.Plural() {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not found"})
		return
	}
	comp, ok := s.reg.Get(kind, parts[1])
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": strings.ToUpper(string(kind[:1])) + string(kind[1:]) + " not found"})
		return
	}
	if s.cfg.Authorize != nil {
		if err := s.cfg.Authorize(r, kind, comp.ID()); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]any{"detail": err.Error()})
			return
		}
	}
	if isCard {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "Method not allowed"})
			return
		}
		s.serveCard(w, r, comp)
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"detail": "Method not allowed"})
		return
	}
	s.serveRPC(w, r, comp)
}

// ---------------------------------------------------------------- agent card

var inputModes = []string{"text/plain", "application/json", "image/*", "audio/*", "video/*", "application/octet-stream"}

// AgentCard builds the card of a component served at url (Python build_agent_card).
func (s *Server) AgentCard(c component.Component, url string) *a2a.AgentCard {
	name := c.Name()
	if name == "" {
		name = c.ID()
	}
	desc := c.Description()
	if desc == "" {
		desc = name
	}
	card := &a2a.AgentCard{
		Name: name, Description: desc, Version: s.cfg.Version,
		SupportedInterfaces: []a2a.AgentInterface{{URL: url, ProtocolBinding: "JSONRPC", ProtocolVersion: a2a.Version10}},
		Capabilities:        a2a.AgentCapabilities{Streaming: true},
		DefaultInputModes:   inputModes,
		DefaultOutputModes:  []string{"text/plain"},
		Skills:              []a2a.AgentSkill{{ID: c.ID(), Name: name, Description: desc, Tags: []string{"agno"}}},
		Provider:            s.cfg.Provider,
		DocumentationURL:    s.cfg.DocumentationURL,
		IconURL:             s.cfg.IconURL,
	}
	if !s.cfg.DisableV03Compat {
		card.SupportedInterfaces = append(card.SupportedInterfaces, a2a.AgentInterface{URL: url, ProtocolBinding: "JSONRPC", ProtocolVersion: a2a.Version03})
		card.URL, card.PreferredTransport, card.ProtocolVersion = url, "JSONRPC", "0.3.0"
	}
	for name, scheme := range s.cfg.SecuritySchemes {
		if card.SecuritySchemes == nil {
			card.SecuritySchemes = map[string]map[string]any{}
		}
		card.SecuritySchemes[name] = scheme
		card.SecurityRequirements = append(card.SecurityRequirements, map[string]any{"schemes": map[string]any{name: map[string]any{"list": []string{}}}})
	}
	return card
}

func (s *Server) endpointURL(r *http.Request, c component.Component) string {
	base := strings.TrimRight(s.cfg.BaseURL, "/")
	if base == "" {
		scheme := "http"
		if r.TLS != nil || strings.Contains(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return fmt.Sprintf("%s%s/%s/%s", base, s.cfg.Prefix, c.Kind().Plural(), c.ID())
}

func (s *Server) serveCard(w http.ResponseWriter, r *http.Request, c component.Component) {
	b, _ := json.Marshal(s.AgentCard(c, s.endpointURL(r, c)))
	sum := sha256.Sum256(b)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

// ---------------------------------------------------------------- JSON-RPC

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func rpcErr(code int, format string, args ...any) *a2a.RPCError {
	return &a2a.RPCError{Code: code, Message: fmt.Sprintf(format, args...)}
}

type call struct {
	id      any
	method  string
	v03     bool
	userID  string
	authed  string
	comp    component.Component
	request *http.Request
}

func (s *Server) serveRPC(w http.ResponseWriter, r *http.Request, comp component.Component) {
	var req a2a.RPCRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, a2a.RPCResponse{JSONRPC: "2.0", Error: rpcErr(a2a.CodeParseError, "Parse error: %v", err)})
		return
	}
	c := &call{id: req.ID, method: req.Method, comp: comp, request: r}
	if s.cfg.Identity != nil {
		c.authed = s.cfg.Identity(r)
	}
	c.userID = c.authed
	if c.userID == "" {
		c.userID = r.Header.Get("X-User-ID")
	}
	switch req.Method {
	case a2a.MethodV03Send, a2a.MethodV03Stream, a2a.MethodV03GetTask, a2a.MethodV03Cancel, a2a.MethodV03Subscribe:
		if s.cfg.DisableV03Compat {
			s.reply(w, c, nil, rpcErr(a2a.CodeMethodNotFound, "Method not found: %s", req.Method))
			return
		}
		c.v03 = true
	}
	switch req.Method {
	case a2a.MethodSendMessage, a2a.MethodV03Send:
		res, err := s.send(r.Context(), c, req.Params, nil)
		s.reply(w, c, res, err)
	case a2a.MethodSendStreamingMessage, a2a.MethodV03Stream:
		s.stream(w, r, c, req.Params)
	case a2a.MethodGetTask, a2a.MethodV03GetTask:
		res, err := s.getTask(r.Context(), c, req.Params)
		s.reply(w, c, res, err)
	case a2a.MethodCancelTask, a2a.MethodV03Cancel:
		res, err := s.cancelTask(r.Context(), c, req.Params)
		s.reply(w, c, res, err)
	case a2a.MethodListTasks:
		res, err := s.listTasks(r.Context(), c, req.Params)
		if err != nil {
			s.reply(w, c, nil, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": c.id, "result": res})
	case a2a.MethodSubscribeToTask, a2a.MethodV03Subscribe:
		s.reply(w, c, nil, rpcErr(a2a.CodeUnsupportedOperation, "Task resubscription is not supported"))
	default:
		s.reply(w, c, nil, rpcErr(a2a.CodeMethodNotFound, "Method not found: %s", req.Method))
	}
}

// encode renders a stream response in the caller's protocol version. v1.0
// GetTask/CancelTask return a bare Task; SendMessage returns the oneof.
func (c *call) encode(res *a2a.StreamResponse) any {
	if c.v03 {
		return res.EncodeV03()
	}
	if res.Task != nil && (c.method == a2a.MethodGetTask || c.method == a2a.MethodCancelTask) {
		return res.Task
	}
	return res
}

func (s *Server) reply(w http.ResponseWriter, c *call, res *a2a.StreamResponse, err error) {
	out := map[string]any{"jsonrpc": "2.0", "id": c.id}
	if err != nil {
		var re *a2a.RPCError
		if !errors.As(err, &re) {
			re = rpcErr(a2a.CodeInternalError, "%v", err)
		}
		out["error"] = re
	} else {
		out["result"] = c.encode(res)
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------------------------------------------------------------- tasks

func (s *Server) lock(taskID string) func() {
	m, _ := s.locks.LoadOrStore(taskID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *Server) load(ctx context.Context, c *call, taskID string) (*TaskRecord, error) {
	rec, err := s.store.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}
	// A task belongs to one component, and to its user when the caller is authenticated.
	if rec == nil || rec.ComponentID != c.comp.ID() || rec.Kind != c.comp.Kind() || (c.authed != "" && rec.UserID != "" && rec.UserID != c.authed) {
		return nil, rpcErr(a2a.CodeTaskNotFound, "Task not found: %s", taskID)
	}
	return rec, nil
}

// update mutates a stored task under its lock and returns the saved copy.
func (s *Server) update(ctx context.Context, taskID string, fn func(*TaskRecord) bool) (*TaskRecord, error) {
	unlock := s.lock(taskID)
	defer unlock()
	rec, err := s.store.Get(context.WithoutCancel(ctx), taskID)
	if err != nil || rec == nil {
		return nil, fmt.Errorf("task %s vanished: %v", taskID, err)
	}
	if !fn(rec) {
		return rec, nil
	}
	return rec, s.store.Save(context.WithoutCancel(ctx), rec)
}

type idParams struct {
	ID            string `json:"id"`
	Name          string `json:"name"` // v1.0 REST-style "tasks/{id}"
	HistoryLength *int   `json:"historyLength"`
}

func (p *idParams) taskID() string {
	if p.ID != "" {
		return p.ID
	}
	return strings.TrimPrefix(p.Name, "tasks/")
}

func trimHistory(t *a2a.Task, n *int) {
	if n == nil || *n < 0 {
		return
	}
	if *n == 0 {
		t.History = nil
	} else if len(t.History) > *n {
		t.History = t.History[len(t.History)-*n:]
	}
}

func (s *Server) getTask(ctx context.Context, c *call, raw json.RawMessage) (*a2a.StreamResponse, error) {
	var p idParams
	if err := json.Unmarshal(raw, &p); err != nil || p.taskID() == "" {
		return nil, rpcErr(a2a.CodeInvalidParams, "Task id is required")
	}
	rec, err := s.load(ctx, c, p.taskID())
	if err != nil {
		return nil, err
	}
	trimHistory(&rec.Task, p.HistoryLength)
	return &a2a.StreamResponse{Task: &rec.Task}, nil
}

func (s *Server) listTasks(ctx context.Context, c *call, raw json.RawMessage) (map[string]any, error) {
	var p struct {
		ContextID     string `json:"contextId"`
		Status        string `json:"status"`
		PageSize      int    `json:"pageSize"`
		PageToken     string `json:"pageToken"`
		HistoryLength *int   `json:"historyLength"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, rpcErr(a2a.CodeInvalidParams, "Invalid params: %v", err)
		}
	}
	q := TaskQuery{ComponentID: c.comp.ID(), Kind: c.comp.Kind(), ContextID: p.ContextID, UserID: c.authed, PageSize: p.PageSize, PageToken: p.PageToken}
	if p.Status != "" {
		q.State = a2a.ParseTaskState(p.Status)
	}
	recs, next, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}
	tasks := make([]a2a.Task, 0, len(recs))
	for _, r := range recs {
		trimHistory(&r.Task, p.HistoryLength)
		tasks = append(tasks, r.Task)
	}
	return map[string]any{"tasks": tasks, "nextPageToken": next, "pageSize": len(tasks), "totalSize": total}, nil
}

func (s *Server) cancelTask(ctx context.Context, c *call, raw json.RawMessage) (*a2a.StreamResponse, error) {
	var p idParams
	if err := json.Unmarshal(raw, &p); err != nil || p.taskID() == "" {
		return nil, rpcErr(a2a.CodeInvalidParams, "Task id is required")
	}
	rec, err := s.load(ctx, c, p.taskID())
	if err != nil {
		return nil, err
	}
	if rec.Task.Status.State.IsTerminal() {
		return nil, rpcErr(a2a.CodeTaskNotCancelable, "Task cannot be canceled: it is %s", rec.Task.Status.State.V03())
	}
	if cancel, ok := s.running.Load(rec.Task.ID); ok {
		cancel.(context.CancelFunc)()
	}
	runID := rec.RunID
	if runID == "" {
		runID = rec.Task.ID
	}
	if _, err := component.Cancel(ctx, c.comp, runID); err != nil && !errors.Is(err, component.ErrNotSupported) {
		return nil, err
	}
	rec, err = s.update(ctx, rec.Task.ID, func(r *TaskRecord) bool {
		if r.Task.Status.State.IsTerminal() {
			return false
		}
		r.Task.Status = a2a.NewStatus(a2a.StateCanceled, agentMessage(r, a2a.TextPart("Run was cancelled")))
		return true
	})
	if err != nil {
		return nil, err
	}
	return &a2a.StreamResponse{Task: &rec.Task}, nil
}

func agentMessage(r *TaskRecord, parts ...a2a.Part) *a2a.Message {
	return &a2a.Message{MessageID: uuid.NewString(), Role: a2a.RoleAgent, TaskID: r.Task.ID, ContextID: r.Task.ContextID, Parts: parts}
}

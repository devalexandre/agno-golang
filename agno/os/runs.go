package os

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/team"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func newUUID() string { return uuid.New().String() }

func (os *AgentOS) findAgent(id string) *agent.Agent {
	for _, a := range os.agents {
		if a.GetName() == id || generateDeterministicID("agent", a.GetName()) == id || (a.GetID() != "" && a.GetID() == id) {
			return a
		}
	}
	return nil
}

func (os *AgentOS) findTeam(id string) *team.Team {
	for _, t := range os.teams {
		if t.GetName() == id || generateDeterministicID("team", t.GetName()) == id {
			return t
		}
	}
	return nil
}

func (os *AgentOS) findWorkflow(id string) *v2.Workflow {
	for _, w := range os.workflows {
		if w.WorkflowID == id || w.Name == id || generateDeterministicID("workflow", w.Name) == id {
			return w
		}
	}
	return nil
}

// componentParam returns the component id path parameter whatever its name.
func componentParam(c *gin.Context) string {
	for _, k := range []string{"id", "agent_id", "team_id", "workflow_id"} {
		if v := c.Param(k); v != "" {
			return v
		}
	}
	return ""
}

// runRequest is the parsed body of POST /{component}/{id}/runs.
type runRequest struct {
	Message    string
	SessionID  string
	UserID     string
	Stream     *bool
	Background bool
}

func parseBoolString(v string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(v))
	return b
}

// parseRunRequest reads JSON, multipart or urlencoded run requests (the
// Python endpoint takes Form fields; JSON is kept for Go clients).
func parseRunRequest(c *gin.Context) (*runRequest, error) {
	req := &runRequest{}
	ct := c.GetHeader("Content-Type")
	if strings.Contains(ct, "application/json") {
		var body struct {
			Message    string `json:"message"`
			Stream     *bool  `json:"stream"`
			SessionID  string `json:"session_id"`
			UserID     string `json:"user_id"`
			Background bool   `json:"background"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			return nil, fmt.Errorf("Invalid JSON: %v", err)
		}
		req.Message, req.Stream, req.SessionID, req.UserID, req.Background = body.Message, body.Stream, body.SessionID, body.UserID, body.Background
		return req, nil
	}
	if strings.Contains(ct, "multipart/form-data") {
		_ = c.Request.ParseMultipartForm(32 << 20)
	} else {
		_ = c.Request.ParseForm()
	}
	req.Message = c.PostForm("message")
	req.SessionID = c.PostForm("session_id")
	req.UserID = c.PostForm("user_id")
	if v, ok := c.GetPostForm("stream"); ok {
		b := parseBoolString(v)
		req.Stream = &b
	}
	req.Background = parseBoolString(c.PostForm("background"))
	return req, nil
}

// runSpec describes one run to execute.
type runSpec struct {
	componentType string // agents, teams, workflows
	componentID   string
	componentName string
	runID         string
	sessionID     string
	userID        string
	message       string
	stream        bool
	background    bool
	auth          *RunAuthorization
	// execute runs the component, publishing events through emit. It must
	// return the final content and metrics.
	execute func(ctx context.Context, rs *runEventStream, emit func(map[string]interface{})) (interface{}, map[string]interface{}, error)
}

func (sp *runSpec) eventPrefix() string {
	if sp.componentType == "teams" {
		return "Team"
	}
	return ""
}

func (sp *runSpec) sessionType() string {
	switch sp.componentType {
	case "teams":
		return SessionTypeTeam
	case "workflows":
		return SessionTypeWorkflow
	}
	return SessionTypeAgent
}

// baseEvent builds an event payload with the component identification.
func (sp *runSpec) baseEvent(name string) map[string]interface{} {
	ev := map[string]interface{}{
		"event":      name,
		"created_at": time.Now().Unix(),
		"run_id":     sp.runID,
		"session_id": sp.sessionID,
	}
	switch sp.componentType {
	case "teams":
		ev["team_id"], ev["team_name"] = sp.componentID, sp.componentName
	case "workflows":
		ev["workflow_id"], ev["workflow_name"] = sp.componentID, sp.componentName
	default:
		ev["agent_id"], ev["agent_name"] = sp.componentID, sp.componentName
	}
	return ev
}

// recordRunStart stores the RUNNING run in the session.
func (os *AgentOS) recordRunStart(ctx context.Context, sp *runSpec) {
	db := os.primaryDB()
	if db == nil {
		return
	}
	os.sessionMu.Lock()
	defer os.sessionMu.Unlock()
	s, _ := db.GetSession(ctx, sp.sessionID)
	now := time.Now().Unix()
	if s == nil {
		s = &SessionRecord{SessionID: sp.sessionID, SessionType: sp.sessionType(), UserID: sp.userID, CreatedAt: now}
		switch sp.componentType {
		case "teams":
			s.TeamID = sp.componentID
		case "workflows":
			s.WorkflowID = sp.componentID
		default:
			s.AgentID = sp.componentID
		}
	}
	run := &RunRecord{
		RunID: sp.runID, RunIndex: len(s.Runs), SessionID: sp.sessionID, UserID: sp.userID,
		Status: string(models.RunStatusRunning), RunInput: sp.message, CreatedAt: now, UpdatedAt: now,
	}
	switch sp.componentType {
	case "teams":
		run.TeamID = sp.componentID
	case "workflows":
		run.WorkflowID = sp.componentID
	default:
		run.AgentID = sp.componentID
	}
	s.Runs = append(s.Runs, run)
	s.UpdatedAt = now
	if err := db.UpsertSession(ctx, s); err != nil {
		log.Printf("AgentOS: failed to record run %s: %v", sp.runID, err)
	}
}

// recordRunEnd stores the final status of a run.
func (os *AgentOS) recordRunEnd(sp *runSpec, rs *runEventStream, content interface{}, metrics map[string]interface{}) {
	db := os.primaryDB()
	if db == nil {
		return
	}
	ctx := context.Background()
	os.sessionMu.Lock()
	defer os.sessionMu.Unlock()
	s, _ := db.GetSession(ctx, sp.sessionID)
	if s == nil {
		return
	}
	events, _, status, _, _ := rs.snapshot(-1)
	rs.mu.Lock()
	stage := rs.stage
	rs.mu.Unlock()
	for _, r := range s.Runs {
		if r.RunID != sp.runID {
			continue
		}
		r.Status = string(status)
		r.CancellationStage = string(stage)
		r.Content = content
		r.Metrics = metrics
		r.UpdatedAt = time.Now().Unix()
		if sp.background && sp.stream {
			r.Events = events
		}
	}
	s.UpdatedAt = time.Now().Unix()
	if err := db.UpsertSession(ctx, s); err != nil {
		log.Printf("AgentOS: failed to record run %s: %v", sp.runID, err)
	}
}

// startRun executes a run detached from the request. The returned stream
// receives every event.
func (os *AgentOS) startRun(sp *runSpec) *runEventStream {
	rs := newRunEventStream(sp.runID, sp.sessionID, sp.componentType, sp.componentID, sp.background)
	ctx, cancel := context.WithCancel(os.ctx)
	if sp.auth != nil {
		ctx = WithRunAuthorization(ctx, sp.auth)
	}
	rs.cancel = cancel
	os.runs.register(rs)
	os.recordRunStart(ctx, sp)

	go func() {
		defer cancel()
		prefix := sp.eventPrefix()
		var content interface{}
		var metrics map[string]interface{}
		var runErr error
		startedAt := time.Now()

		rs.mu.Lock()
		cancelledBeforeStart := rs.cancelled
		rs.started = true
		rs.mu.Unlock()

		finishCancelled := func(stage models.CancellationStage) {
			ev := sp.baseEvent(prefix + "RunCancelled")
			if sp.componentType == "workflows" {
				ev["event"] = "WorkflowCancelled"
			}
			ev["cancellation_stage"] = string(stage)
			rs.publish(ev)
			rs.complete(models.RunStatusCancelled, stage)
		}

		if cancelledBeforeStart {
			finishCancelled(models.CancellationStagePending)
			os.recordRunEnd(sp, rs, nil, nil)
			return
		}
		rs.setStatus(models.RunStatusRunning)

		func() {
			defer func() {
				if r := recover(); r != nil {
					runErr = fmt.Errorf("run panicked: %v", r)
				}
			}()
			content, metrics, runErr = sp.execute(ctx, rs, rs.publish)
		}()

		if metrics == nil {
			metrics = map[string]interface{}{}
		}
		if _, ok := metrics["duration"]; !ok {
			metrics["duration"] = time.Since(startedAt).Seconds()
		}

		rs.mu.Lock()
		wasCancelled := rs.cancelled
		rs.mu.Unlock()
		switch {
		case wasCancelled || errors.Is(runErr, context.Canceled):
			finishCancelled(models.CancellationStageExecuting)
		case runErr != nil:
			ev := sp.baseEvent(prefix + "RunError")
			if sp.componentType == "workflows" {
				ev["event"] = "WorkflowError"
			}
			ev["content"] = runErr.Error()
			ev["error"] = runErr.Error()
			rs.publish(ev)
			rs.complete(models.RunStatusError, "")
		default:
			if sp.componentType != "workflows" {
				rs.publish(sp.baseEvent(prefix + "RunContentCompleted"))
			}
			name := prefix + "RunCompleted"
			if sp.componentType == "workflows" {
				name = "WorkflowCompleted"
			}
			ev := sp.baseEvent(name)
			ev["content"] = content
			ev["content_type"] = "str"
			ev["metrics"] = metrics
			rs.publish(ev)
			rs.complete(models.RunStatusCompleted, "")
		}
		os.recordRunEnd(sp, rs, content, metrics)
	}()
	return rs
}

// streamRunEvents writes events with index > lastIndex as SSE until the run
// finishes or the client goes away. It returns false on disconnect.
func streamRunEvents(c *gin.Context, rs *runEventStream, lastIndex int) bool {
	ctx := c.Request.Context()
	for {
		events, done, _, changed, _ := rs.snapshot(lastIndex)
		for _, ev := range events {
			if _, err := c.Writer.Write(formatSSE(ev)); err != nil {
				return false
			}
			lastIndex = ev["event_index"].(int)
		}
		c.Writer.Flush()
		if done {
			return true
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return false
		case <-time.After(sseKeepaliveInterval):
			if _, err := c.Writer.Write([]byte(": keepalive\n\n")); err != nil {
				return false
			}
			c.Writer.Flush()
		}
	}
}

func setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
}

// serveRun executes a run and answers the request in the shape the request
// asked for (Python agents/teams/workflows routers):
//   - stream=true:                         SSE of the run; disconnect cancels it
//   - stream=true,  background=true:       resumable SSE; the run survives disconnects
//   - stream=false:                        JSON run output
//   - stream=false, background=true:       202 with the run metadata; poll the run
func (os *AgentOS) serveRun(c *gin.Context, sp *runSpec) {
	if sp.background && os.primaryDB() == nil {
		httpError(c, http.StatusBadRequest, "Background execution requires a database to be configured")
		return
	}
	rs := os.startRun(sp)

	if sp.stream {
		setSSEHeaders(c)
		finished := streamRunEvents(c, rs, -1)
		if !finished && !sp.background {
			rs.requestCancel()
		}
		c.Abort()
		return
	}

	if sp.background {
		c.JSON(http.StatusAccepted, gin.H{
			"run_id":     sp.runID,
			"session_id": sp.sessionID,
			"status":     string(models.RunStatusPending),
			"created_at": time.Now().Unix(),
		})
		return
	}

	// Wait for completion.
	for {
		_, done, _, changed, _ := rs.snapshot(1 << 30)
		if done {
			break
		}
		select {
		case <-changed:
		case <-c.Request.Context().Done():
			rs.requestCancel()
			return
		}
	}
	events, _, status, _, _ := rs.snapshot(-1)
	last := events[len(events)-1]
	body := gin.H{
		"run_id":     sp.runID,
		"session_id": sp.sessionID,
		"status":     string(status),
		"created_at": last["created_at"],
	}
	for _, k := range []string{"agent_id", "agent_name", "team_id", "team_name", "workflow_id", "workflow_name", "content", "content_type", "metrics", "cancellation_stage"} {
		if v, ok := last[k]; ok {
			body[k] = v
		}
	}
	if status == models.RunStatusError {
		body["error"] = last["error"]
		c.JSON(http.StatusInternalServerError, body)
		return
	}
	c.JSON(http.StatusOK, body)
}

// buildRunSpec fills the common fields of a run from the request.
func (os *AgentOS) buildRunSpec(c *gin.Context, req *runRequest, componentType, componentID, componentName string, defaultStream bool) *runSpec {
	sp := &runSpec{
		componentType: componentType,
		componentID:   componentID,
		componentName: componentName,
		runID:         newUUID(),
		sessionID:     req.SessionID,
		userID:        req.UserID,
		message:       req.Message,
		stream:        defaultStream,
		background:    req.Background,
		auth:          requestAuthorization(c),
	}
	if req.Stream != nil {
		sp.stream = *req.Stream
	}
	if sp.auth != nil && sp.auth.Context.UserID != "" {
		sp.userID = sp.auth.Context.UserID
	}
	if sp.sessionID == "" {
		sp.sessionID = newUUID()
	}
	return sp
}

// agentRunsHandler handles POST /agents/{agent_id}/runs.
func (os *AgentOS) agentRunsHandler(c *gin.Context) {
	agentID := componentParam(c)
	req, err := parseRunRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error(), "error": err.Error(), "details": "Expected format: {\"message\": \"your message\"}"})
		return
	}
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Message is required", "error": "Message is required", "details": "Please provide a 'message' field"})
		return
	}
	target := os.findAgent(agentID)
	if target == nil {
		httpError(c, http.StatusNotFound, "Agent not found")
		return
	}

	// Media uploads (multipart).
	var mediaOpts []agent.RunOption
	if form, ferr := c.MultipartForm(); ferr == nil && form != nil {
		if files, ok := form.File["images"]; ok && len(files) > 0 {
			imgs, err := processImageUploads(files)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to process image uploads", "detail": err.Error()})
				return
			}
			vals := make([]agent.Image, 0, len(imgs))
			for _, i := range imgs {
				vals = append(vals, *i)
			}
			mediaOpts = append(mediaOpts, agent.WithImages(vals...))
		}
		if files, ok := form.File["audio"]; ok && len(files) > 0 {
			auds, err := processAudioUploads(files)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to process audio uploads", "detail": err.Error()})
				return
			}
			vals := make([]agent.Audio, 0, len(auds))
			for _, a := range auds {
				vals = append(vals, *a)
			}
			mediaOpts = append(mediaOpts, agent.WithAudio(vals...))
		}
		if files, ok := form.File["videos"]; ok && len(files) > 0 {
			vids, err := processVideoUploads(files)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to process video uploads", "detail": err.Error()})
				return
			}
			vals := make([]agent.Video, 0, len(vids))
			for _, v := range vids {
				vals = append(vals, *v)
			}
			mediaOpts = append(mediaOpts, agent.WithVideos(vals...))
		}
		if files, ok := form.File["files"]; ok && len(files) > 0 {
			docs, err := processFileUploads(files)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to process file uploads", "detail": err.Error()})
				return
			}
			vals := make([]agent.File, 0, len(docs))
			for _, f := range docs {
				vals = append(vals, *f)
			}
			mediaOpts = append(mediaOpts, agent.WithFiles(vals...))
		}
	}

	sp := os.buildRunSpec(c, req, "agents", agentID, target.GetName(), true)
	sp.execute = func(ctx context.Context, rs *runEventStream, emit func(map[string]interface{})) (interface{}, map[string]interface{}, error) {
		emit(sp.baseEvent("RunStarted"))
		start := time.Now()
		if sp.stream && len(mediaOpts) == 0 {
			// RunStream has no run options: cancellation stops the stream at
			// the next chunk.
			var full strings.Builder
			err := target.RunStream(sp.message, func(chunk []byte) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				full.Write(chunk)
				ev := sp.baseEvent("RunContent")
				ev["content"] = string(chunk)
				ev["content_type"] = "str"
				ev["reasoning_content"] = ""
				emit(ev)
				return nil
			})
			if err == nil && ctx.Err() != nil {
				err = ctx.Err()
			}
			return full.String(), metricsMap(runCompletedMetrics(nil, start)), err
		}
		opts := []interface{}{agent.WithRunID(sp.runID), agent.WithSessionID(sp.sessionID)}
		if sp.userID != "" {
			opts = append(opts, agent.WithUserID(sp.userID))
		}
		for _, o := range mediaOpts {
			opts = append(opts, o)
		}
		// Propagate an OS cancellation to the agent's own run tracking.
		stop := context.AfterFunc(ctx, func() { target.CancelRun(sp.runID) })
		defer stop()
		resp, err := target.Run(sp.message, opts...)
		if err == nil && resp.Status == models.RunStatusCancelled {
			err = context.Canceled
		}
		if err == nil {
			ev := sp.baseEvent("RunContent")
			ev["content"] = resp.TextContent
			ev["content_type"] = "str"
			ev["reasoning_content"] = ""
			emit(ev)
		}
		return resp.TextContent, metricsMap(runCompletedMetrics(&resp, start)), err
	}
	os.serveRun(c, sp)
}

// teamRunsHandler handles POST /teams/{team_id}/runs.
func (os *AgentOS) teamRunsHandler(c *gin.Context) {
	teamID := componentParam(c)
	req, err := parseRunRequest(c)
	if err != nil {
		httpError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Message == "" {
		httpError(c, http.StatusBadRequest, "Message is required")
		return
	}
	target := os.findTeam(teamID)
	if target == nil {
		httpError(c, http.StatusNotFound, "Team not found")
		return
	}
	sp := os.buildRunSpec(c, req, "teams", teamID, target.GetName(), true)
	sp.execute = func(ctx context.Context, rs *runEventStream, emit func(map[string]interface{})) (interface{}, map[string]interface{}, error) {
		emit(sp.baseEvent("TeamRunStarted"))
		start := time.Now()
		var full strings.Builder
		err := target.RunStream(sp.message, func(chunk []byte) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			full.Write(chunk)
			ev := sp.baseEvent("TeamRunContent")
			ev["content"] = string(chunk)
			ev["content_type"] = "str"
			emit(ev)
			return nil
		})
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		return full.String(), metricsMap(runCompletedMetrics(nil, start)), err
	}
	os.serveRun(c, sp)
}

// workflowRunsHandler handles POST /workflows/{workflow_id}/runs.
func (os *AgentOS) workflowRunsHandler(c *gin.Context) {
	workflowID := componentParam(c)
	req, err := parseRunRequest(c)
	if err != nil {
		httpError(c, http.StatusBadRequest, err.Error())
		return
	}
	target := os.findWorkflow(workflowID)
	if target == nil {
		httpError(c, http.StatusNotFound, "Workflow not found")
		return
	}
	if req.Message == "" {
		httpError(c, http.StatusBadRequest, "Message is required")
		return
	}
	sp := os.buildRunSpec(c, req, "workflows", workflowID, target.Name, true)
	sp.execute = func(ctx context.Context, rs *runEventStream, emit func(map[string]interface{})) (interface{}, map[string]interface{}, error) {
		emit(sp.baseEvent("WorkflowStarted"))
		start := time.Now()

		// Workflow instances keep per-run state, so runs of one workflow are
		// serialized. Step events reach this run through the sink.
		lock := os.workflowLock(target)
		lock.Lock()
		defer lock.Unlock()
		os.setWorkflowSink(target, func(ev *v2.WorkflowRunResponseEvent) {
			payload := sp.baseEvent(string(ev.Event))
			for k, v := range ev.Metadata {
				if _, exists := payload[k]; !exists {
					payload[k] = v
				}
			}
			if ev.Data != nil {
				payload["content"] = jsonClone(ev.Data)
			}
			emit(payload)
		})
		defer os.setWorkflowSink(target, nil)

		resp, err := target.Run(ctx, sp.message)
		if err == nil && ctx.Err() != nil {
			err = ctx.Err()
		}
		var content interface{}
		if resp != nil {
			content = resp.Content
		}
		return content, map[string]interface{}{"duration": time.Since(start).Seconds()}, err
	}
	os.serveRun(c, sp)
}

// workflowEventNames are forwarded from a workflow to the current run sink.
// WorkflowStarted/Completed are emitted by the OS with the API run id.
var workflowEventNames = []v2.WorkflowRunEvent{
	v2.StepStartedEvent, v2.StepCompletedEvent, v2.StepOutputEvent,
	v2.StepsExecutionStartedEvent, v2.StepsExecutionCompletedEvent,
	v2.LoopExecutionStartedEvent, v2.LoopExecutionCompletedEvent,
	v2.LoopIterationStartedEvent, v2.LoopIterationCompletedEvent,
	v2.ParallelExecutionStartedEvent, v2.ParallelExecutionCompletedEvent,
	v2.ConditionExecutionStartedEvent, v2.ConditionExecutionCompletedEvent,
	v2.RouterExecutionStartedEvent, v2.RouterExecutionCompletedEvent,
	v2.StepProgressEvent,
}

// hookWorkflowEvents registers, once per workflow, handlers that forward
// workflow events to the run currently executing it.
func (os *AgentOS) hookWorkflowEvents() {
	for _, w := range os.workflows {
		w := w
		for _, name := range workflowEventNames {
			func() {
				// Workflows built as struct literals have no handler map;
				// they still run, just without step events.
				defer func() { _ = recover() }()
				w.OnEvent(name, func(ev *v2.WorkflowRunResponseEvent) {
					if sink := os.workflowSink(w); sink != nil {
						sink(ev)
					}
				})
			}()
		}
	}
}

func (os *AgentOS) workflowLock(w *v2.Workflow) *sync.Mutex {
	os.wfMu.Lock()
	defer os.wfMu.Unlock()
	if os.wfLocks == nil {
		os.wfLocks = map[*v2.Workflow]*sync.Mutex{}
	}
	if l, ok := os.wfLocks[w]; ok {
		return l
	}
	l := &sync.Mutex{}
	os.wfLocks[w] = l
	return l
}

func (os *AgentOS) setWorkflowSink(w *v2.Workflow, sink func(*v2.WorkflowRunResponseEvent)) {
	os.wfMu.Lock()
	defer os.wfMu.Unlock()
	if os.wfSinks == nil {
		os.wfSinks = map[*v2.Workflow]func(*v2.WorkflowRunResponseEvent){}
	}
	if sink == nil {
		delete(os.wfSinks, w)
		return
	}
	os.wfSinks[w] = sink
}

func (os *AgentOS) workflowSink(w *v2.Workflow) func(*v2.WorkflowRunResponseEvent) {
	os.wfMu.Lock()
	defer os.wfMu.Unlock()
	return os.wfSinks[w]
}

// componentExists reports whether the path's component exists.
func (os *AgentOS) componentExists(componentType, id string) bool {
	switch componentType {
	case "agents":
		return os.findAgent(id) != nil
	case "teams":
		return os.findTeam(id) != nil
	case "workflows":
		return os.findWorkflow(id) != nil
	}
	return false
}

func componentTypeFromPath(c *gin.Context) string {
	t, _ := ResourceContextFromPath(c.Request.URL.Path)
	return t
}

func componentLabel(componentType string) string {
	switch componentType {
	case "teams":
		return "Team"
	case "workflows":
		return "Workflow"
	}
	return "Agent"
}

// cancelRunHandler handles POST /{agents|teams|workflows}/{id}/runs/{run_id}/cancel.
// As in Python, cancellation intent is always recorded (a run that has not
// started yet is cancelled at PENDING stage) and the response is 200 {}.
func (os *AgentOS) cancelRunHandler(c *gin.Context) {
	componentType := componentTypeFromPath(c)
	id := componentParam(c)
	if !os.componentExists(componentType, id) {
		httpError(c, http.StatusNotFound, componentLabel(componentType)+" not found")
		return
	}
	runID := c.Param("run_id")
	if rs := os.runs.get(runID); rs != nil && !resourceMatches(os, componentType, rs.componentID, id) {
		httpError(c, http.StatusNotFound, "Run not found")
		return
	}
	os.runs.cancel(runID)
	if componentType == "agents" {
		os.findAgent(id).CancelRun(runID)
	}
	c.JSON(http.StatusOK, gin.H{})
}

func resourceMatches(os *AgentOS, componentType, a, b string) bool {
	if a == b {
		return true
	}
	return containsString(os.resourceAliases(componentType, a), b)
}

// resumeRunHandler handles POST /{agents|teams|workflows}/{id}/runs/{run_id}/resume.
// last_event_index (form, JSON or query) is the index of the last event the
// client received; ?from=N asks for events starting at index N.
func (os *AgentOS) resumeRunHandler(c *gin.Context) {
	componentType := componentTypeFromPath(c)
	id := componentParam(c)
	if !os.componentExists(componentType, id) {
		httpError(c, http.StatusNotFound, componentLabel(componentType)+" not found")
		return
	}
	runID := c.Param("run_id")

	lastIndex := -1
	sessionID := c.Query("session_id")
	if v := c.Query("from"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			httpError(c, http.StatusUnprocessableEntity, "Invalid value for 'from'")
			return
		}
		lastIndex = n - 1
	}
	raw := c.Query("last_event_index")
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			LastEventIndex *int   `json:"last_event_index"`
			SessionID      string `json:"session_id"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			if body.LastEventIndex != nil {
				raw = strconv.Itoa(*body.LastEventIndex)
			}
			if body.SessionID != "" {
				sessionID = body.SessionID
			}
		}
	} else {
		_ = c.Request.ParseMultipartForm(1 << 20)
		if v := c.PostForm("last_event_index"); v != "" {
			raw = v
		}
		if v := c.PostForm("session_id"); v != "" {
			sessionID = v
		}
	}
	if raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpError(c, http.StatusUnprocessableEntity, "Invalid value for 'last_event_index'")
			return
		}
		lastIndex = n
	}
	if scoped := scopedUserID(c); scoped != "" && sessionID == "" {
		httpError(c, http.StatusBadRequest, "session_id is required")
		return
	}

	rs := os.runs.get(runID)
	if rs != nil && !resourceMatches(os, componentType, rs.componentID, id) {
		rs = nil
	}
	setSSEHeaders(c)
	defer c.Abort()

	writeFrame := func(name string, payload map[string]interface{}) {
		payload["event"] = name
		_, _ = c.Writer.Write(formatSSE(payload))
		c.Writer.Flush()
	}

	if rs == nil {
		// PATH 3: fall back to the database.
		if sessionID != "" {
			if db := os.primaryDB(); db != nil {
				if s, _ := db.GetSession(c.Request.Context(), sessionID); s != nil {
					for _, r := range s.Runs {
						if r.RunID != runID {
							continue
						}
						missed := make([]map[string]interface{}, 0)
						for _, ev := range r.Events {
							if idx, ok := toFloat(ev["event_index"]); ok && int(idx) <= lastIndex {
								continue
							}
							missed = append(missed, ev)
						}
						writeFrame("replay", map[string]interface{}{
							"run_id": runID, "status": r.Status, "total_events": len(missed),
							"total_buffered": len(r.Events), "last_event_index_requested": lastIndex,
							"message": fmt.Sprintf("Run %s. Replaying %d events from the database.", strings.ToLower(r.Status), len(missed)),
						})
						for _, ev := range missed {
							_, _ = c.Writer.Write(formatSSE(ev))
						}
						c.Writer.Flush()
						return
					}
				}
			}
		}
		writeFrame("error", map[string]interface{}{"error": fmt.Sprintf("Run %s not found in buffer or database", runID)})
		return
	}

	missed, done, status, _, total := rs.snapshot(lastIndex)
	if done {
		// PATH 2: finished, replay what was missed.
		writeFrame("replay", map[string]interface{}{
			"run_id": runID, "status": string(status), "total_events": len(missed), "total_buffered": total,
			"last_event_index_requested": lastIndex,
			"message":                    fmt.Sprintf("Run %s. Replaying %d missed events (of %d total).", strings.ToLower(string(status)), len(missed), total),
		})
		for _, ev := range missed {
			_, _ = c.Writer.Write(formatSSE(ev))
		}
		c.Writer.Flush()
		return
	}
	// PATH 1: still active, catch up then tail.
	if len(missed) > 0 {
		writeFrame("catch_up", map[string]interface{}{
			"run_id": runID, "status": "running", "missed_events": len(missed), "current_event_count": total,
		})
	}
	streamRunEvents(c, rs, lastIndex)
}

// getRunHandler handles GET /{agents|teams|workflows}/{id}/runs/{run_id}.
func (os *AgentOS) getRunHandler(c *gin.Context) {
	componentType := componentTypeFromPath(c)
	id := componentParam(c)
	if !os.componentExists(componentType, id) {
		httpError(c, http.StatusNotFound, componentLabel(componentType)+" not found")
		return
	}
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	q := SessionQuery{UserID: scopedUserID(c)}
	if sid := c.Query("session_id"); sid != "" {
		q.Filter = func(s *SessionRecord) bool { return s.SessionID == sid }
	}
	sessions, _, err := db.ListSessions(c.Request.Context(), q)
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	for _, s := range sessions {
		for _, r := range s.Runs {
			if r.RunID == runID && resourceMatches(os, componentType, runComponentID(componentType, r), id) {
				c.JSON(http.StatusOK, runSchema(r))
				return
			}
		}
	}
	httpError(c, http.StatusNotFound, "Run not found")
}

// listRunsHandler handles GET /{agents|teams|workflows}/{id}/runs.
func (os *AgentOS) listRunsHandler(c *gin.Context) {
	componentType := componentTypeFromPath(c)
	id := componentParam(c)
	if !os.componentExists(componentType, id) {
		httpError(c, http.StatusNotFound, componentLabel(componentType)+" not found")
		return
	}
	db, ok := os.dbFromRequest(c)
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", 20, 1)
	if !ok {
		return
	}
	page, ok := queryInt(c, "page", 1, 1)
	if !ok {
		return
	}
	q := SessionQuery{UserID: scopedUserID(c)}
	if sid := c.Query("session_id"); sid != "" {
		q.Filter = func(s *SessionRecord) bool { return s.SessionID == sid }
	}
	sessions, _, err := db.ListSessions(c.Request.Context(), q)
	if err != nil {
		httpError(c, http.StatusInternalServerError, err.Error())
		return
	}
	statuses := map[string]bool{}
	for _, st := range c.QueryArray("status") {
		statuses[strings.ToUpper(st)] = true
	}
	runs := make([]*RunRecord, 0)
	for _, s := range sessions {
		for _, r := range s.Runs {
			if r.ParentRunID != "" || !resourceMatches(os, componentType, runComponentID(componentType, r), id) {
				continue
			}
			if len(statuses) > 0 && !statuses[strings.ToUpper(r.Status)] {
				continue
			}
			runs = append(runs, r)
		}
	}
	// Newest first (insertion sort keeps equal timestamps stable).
	sortRunsDesc(runs)
	total := len(runs)
	data := make([]gin.H, 0)
	for _, r := range paginate(runs, limit, page) {
		data = append(data, runSchema(r))
	}
	c.JSON(http.StatusOK, paginatedResponse(data, page, limit, total))
}

func sortRunsDesc(runs []*RunRecord) {
	for i := 1; i < len(runs); i++ {
		for j := i; j > 0 && runs[j].CreatedAt > runs[j-1].CreatedAt; j-- {
			runs[j], runs[j-1] = runs[j-1], runs[j]
		}
	}
}

func runComponentID(componentType string, r *RunRecord) string {
	switch componentType {
	case "teams":
		return r.TeamID
	case "workflows":
		return r.WorkflowID
	}
	return r.AgentID
}

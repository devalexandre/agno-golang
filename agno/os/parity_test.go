package os

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// slowModel streams `chunks` chunks, pausing between them, so runs can be
// cancelled and resumed while active.
type slowModel struct {
	stubModel
	chunks int
	delay  time.Duration
}

func (m slowModel) InvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range opts {
		o(call)
	}
	for i := 0; i < m.chunks; i++ {
		time.Sleep(m.delay)
		if call.StreamingFunc != nil {
			if err := call.StreamingFunc(ctx, []byte(fmt.Sprintf("chunk%d ", i))); err != nil {
				return err
			}
		}
	}
	return nil
}

func newParityOS(t *testing.T, opts AgentOSOptions) (*AgentOS, *gin.Engine) {
	t.Helper()
	if opts.OSID == "" {
		opts.OSID = "parity-os"
	}
	if opts.Settings == nil {
		opts.Settings = &AgentOSSettings{EnableCORS: true}
	}
	if opts.Agents == nil {
		a, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: stubModel{}, Name: "echo"})
		require.NoError(t, err)
		slow, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: slowModel{chunks: 20, delay: 30 * time.Millisecond}, Name: "slow"})
		require.NoError(t, err)
		opts.Agents = []*agent.Agent{a, slow}
	}
	o, err := NewAgentOS(opts)
	require.NoError(t, err)
	return o, o.GetApp()
}

func doJSON(r http.Handler, method, path string, body interface{}, headers ...string) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type sseEvent struct {
	Name string
	Data map[string]interface{}
}

func parseSSE(body string) []sseEvent {
	var out []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.Data)
		case line == "" && cur.Name != "":
			out = append(out, cur)
			cur = sseEvent{}
		}
	}
	return out
}

func TestAgentRunNonStreamAndRunRoutes(t *testing.T) {
	_, r := newParityOS(t, AgentOSOptions{})
	w := doJSON(r, "POST", "/agents/echo/runs", map[string]interface{}{"message": "hi there", "stream": false, "session_id": "s1"})
	require.Equal(t, 200, w.Code, w.Body.String())
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "hi there", out["content"])
	assert.Equal(t, "COMPLETED", out["status"])
	runID := out["run_id"].(string)

	// The run is recorded in the session and readable through the run routes.
	w = doJSON(r, "GET", "/agents/echo/runs/"+runID, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	w = doJSON(r, "GET", "/sessions/s1/runs", nil)
	require.Equal(t, 200, w.Code)
	var runs []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runs))
	require.Len(t, runs, 1)
	assert.Equal(t, "COMPLETED", runs[0]["status"])
	assert.Equal(t, "hi there", runs[0]["run_input"])

	// Cursor pagination over runs.
	doJSON(r, "POST", "/agents/echo/runs", map[string]interface{}{"message": "second", "stream": false, "session_id": "s1"})
	doJSON(r, "POST", "/agents/echo/runs", map[string]interface{}{"message": "third", "stream": false, "session_id": "s1"})
	w = doJSON(r, "GET", "/sessions/s1/runs?limit=2", nil)
	require.Equal(t, 200, w.Code)
	var page struct {
		Data []map[string]interface{} `json:"data"`
		Meta map[string]interface{}   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Data, 2)
	assert.Equal(t, true, page.Meta["has_more"])
	assert.Equal(t, float64(1), page.Meta["oldest_run_index"])
	w = doJSON(r, "GET", "/sessions/s1/runs?limit=2&before_run_index=1", nil)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	require.Len(t, page.Data, 1)
	assert.Equal(t, "hi there", page.Data[0]["run_input"])
	assert.Equal(t, 422, doJSON(r, "GET", "/sessions/s1/runs?before_run_index=1", nil).Code)
}

func TestBackgroundRunResumeAndCancel(t *testing.T) {
	o, r := newParityOS(t, AgentOSOptions{})

	// background=true, stream=false: 202 and the run executes detached.
	w := doJSON(r, "POST", "/agents/echo/runs", map[string]interface{}{"message": "bg", "stream": false, "background": true, "session_id": "bg-s"})
	require.Equal(t, http.StatusAccepted, w.Code)
	var accepted map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	runID := accepted["run_id"].(string)
	require.Eventually(t, func() bool {
		w := doJSON(r, "GET", "/agents/echo/runs/"+runID, nil)
		return w.Code == 200 && strings.Contains(w.Body.String(), `"COMPLETED"`)
	}, 2*time.Second, 10*time.Millisecond)

	// background=true, stream=true: a client that disconnects can resume.
	ctx, cancel := context.WithCancel(context.Background())
	body, _ := json.Marshal(map[string]interface{}{"message": "go", "stream": true, "background": true, "session_id": "bg-s2"})
	req := httptest.NewRequest("POST", "/agents/slow/runs", bytes.NewBuffer(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { r.ServeHTTP(rec, req); close(done) }()
	time.Sleep(150 * time.Millisecond)
	cancel() // client disconnects
	<-done

	var bgRun *runEventStream
	o.runs.mu.Lock()
	for _, rs := range o.runs.runs {
		if rs.sessionID == "bg-s2" {
			bgRun = rs
		}
	}
	o.runs.mu.Unlock()
	require.NotNil(t, bgRun)

	// The run survives the disconnect; resume from event index 2.
	resume := doJSON(r, "POST", "/agents/slow/runs/"+bgRun.runID+"/resume?from=2", nil)
	require.Equal(t, 200, resume.Code)
	events := parseSSE(resume.Body.String())
	require.NotEmpty(t, events)
	var indexes []int
	var completed bool
	for _, ev := range events {
		if idx, ok := ev.Data["event_index"].(float64); ok {
			indexes = append(indexes, int(idx))
		}
		if ev.Name == "RunCompleted" {
			completed = true
			assert.Contains(t, ev.Data["content"], "chunk19")
		}
	}
	require.NotEmpty(t, indexes)
	assert.Equal(t, 2, indexes[0], "resume starts at the requested index")
	assert.True(t, completed)

	// Replaying a finished run with last_event_index sends a replay frame.
	resume = doJSON(r, "POST", "/agents/slow/runs/"+bgRun.runID+"/resume", map[string]interface{}{"last_event_index": 0})
	events = parseSSE(resume.Body.String())
	require.Equal(t, "replay", events[0].Name)
	assert.Equal(t, "COMPLETED", events[0].Data["status"])

	// Cancel a running background run: stage EXECUTING.
	// (a streaming background run whose client detaches right away)
	ctx3, cancel3 := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel3()
	body3, _ := json.Marshal(map[string]interface{}{"message": "x", "stream": true, "background": true, "session_id": "bg-s3"})
	req3 := httptest.NewRequest("POST", "/agents/slow/runs", bytes.NewBuffer(body3)).WithContext(ctx3)
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req3)
	first := parseSSE(rec3.Body.String())
	require.NotEmpty(t, first)
	slowRun := first[0].Data["run_id"].(string)
	w = doJSON(r, "POST", "/agents/slow/runs/"+slowRun+"/cancel", nil)
	require.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{}`, w.Body.String())
	require.Eventually(t, func() bool {
		w := doJSON(r, "GET", "/sessions/bg-s3/runs/"+slowRun, nil)
		return strings.Contains(w.Body.String(), `"CANCELLED"`) && strings.Contains(w.Body.String(), `"EXECUTING"`)
	}, 3*time.Second, 20*time.Millisecond)
}

func TestCancelBeforeStartRecordsPendingStage(t *testing.T) {
	o, _ := newParityOS(t, AgentOSOptions{})
	o.runs.cancel("early-run")
	executed := false
	sp := &runSpec{componentType: "agents", componentID: "echo", runID: "early-run", sessionID: "early-s",
		execute: func(context.Context, *runEventStream, func(map[string]interface{})) (interface{}, map[string]interface{}, error) {
			executed = true
			return nil, nil, nil
		}}
	rs := o.startRun(sp)
	require.Eventually(t, func() bool { _, done, _, _, _ := rs.snapshot(-1); return done }, time.Second, 5*time.Millisecond)
	events, _, status, _, _ := rs.snapshot(-1)
	assert.False(t, executed)
	assert.Equal(t, models.RunStatusCancelled, status)
	assert.Equal(t, "RunCancelled", events[len(events)-1]["event"])
	assert.Equal(t, "PENDING", events[len(events)-1]["cancellation_stage"])
}

func TestWorkflowRunExecutes(t *testing.T) {
	step, err := v2.NewStep(v2.WithName("upper"), v2.WithExecutor(func(in *v2.StepInput) (*v2.StepOutput, error) {
		return &v2.StepOutput{Content: strings.ToUpper(in.GetMessageAsString())}, nil
	}))
	require.NoError(t, err)
	wf := v2.NewWorkflow(v2.WithWorkflowName("shout"), v2.WithWorkflowID("shout"), v2.WithWorkflowSteps([]interface{}{step}))
	_, r := newParityOS(t, AgentOSOptions{Workflows: []*v2.Workflow{wf}})

	w := doJSON(r, "POST", "/workflows/shout/runs", map[string]interface{}{"message": "hello", "stream": false})
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "HELLO")

	w = doJSON(r, "POST", "/workflows/shout/runs", map[string]interface{}{"message": "again", "stream": true})
	require.Equal(t, 200, w.Code)
	names := []string{}
	for _, ev := range parseSSE(w.Body.String()) {
		names = append(names, ev.Name)
	}
	assert.Equal(t, "WorkflowStarted", names[0])
	assert.Equal(t, "WorkflowCompleted", names[len(names)-1])
	assert.Contains(t, w.Body.String(), "AGAIN")
}

func TestSessionSearchAndFilterSchema(t *testing.T) {
	_, r := newParityOS(t, AgentOSOptions{})
	for _, s := range []map[string]interface{}{
		{"session_id": "a1", "user_id": "alice", "agent_id": "echo", "session_name": "Billing question"},
		{"session_id": "a2", "user_id": "bob", "agent_id": "echo", "session_name": "Refund"},
		{"session_id": "t1", "user_id": "alice", "team_id": "t", "session_name": "Team chat"},
	} {
		typ := "agent"
		if _, ok := s["team_id"]; ok {
			typ = "team"
		}
		require.Equal(t, 201, doJSON(r, "POST", "/sessions?type="+typ, s).Code)
	}
	w := doJSON(r, "GET", "/sessions/filter-schema", nil)
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"session_type"`)

	search := func(filter interface{}) []string {
		w := doJSON(r, "POST", "/sessions/search", map[string]interface{}{"filter": filter, "sort_order": "asc"})
		require.Equal(t, 200, w.Code, w.Body.String())
		var out struct {
			Data []map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		ids := []string{}
		for _, d := range out.Data {
			ids = append(ids, d["session_id"].(string))
		}
		return ids
	}
	assert.ElementsMatch(t, []string{"a1", "t1"}, search(map[string]interface{}{"op": "EQ", "key": "user_id", "value": "alice"}))
	assert.ElementsMatch(t, []string{"a1"}, search(map[string]interface{}{"op": "AND", "conditions": []interface{}{
		map[string]interface{}{"op": "EQ", "key": "user_id", "value": "alice"},
		map[string]interface{}{"op": "CONTAINS", "key": "session_name", "value": "billing"},
	}}))
	// NULL semantics: sessions without a team_id match neither EQ nor NOT EQ.
	assert.ElementsMatch(t, []string{}, search(map[string]interface{}{"op": "NOT", "condition": map[string]interface{}{"op": "EQ", "key": "team_id", "value": "t"}}))
	assert.ElementsMatch(t, []string{"a1", "a2"}, search(map[string]interface{}{"op": "IN", "key": "session_type", "values": []interface{}{"agent"}}))

	w = doJSON(r, "POST", "/sessions/search", map[string]interface{}{"filter": map[string]interface{}{"op": "EQ", "key": "bogus", "value": "x"}})
	assert.Equal(t, 400, w.Code)

	// Rename + bulk delete.
	w = doJSON(r, "POST", "/sessions/a2/rename", map[string]interface{}{"session_name": "Renamed"})
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "Renamed")
	w = doJSON(r, "DELETE", "/sessions", map[string]interface{}{"session_ids": []string{"a1", "a2"}, "session_types": []string{"agent", "agent"}})
	assert.Equal(t, 204, w.Code)
	w = doJSON(r, "GET", "/sessions?type=agent", nil)
	assert.Contains(t, w.Body.String(), `"total_count":0`)
}

func TestDBSelection(t *testing.T) {
	_, r := newParityOS(t, AgentOSOptions{Databases: []Database{NewInMemoryDB("one"), NewInMemoryDB("two")}})
	assert.Equal(t, 400, doJSON(r, "GET", "/memories", nil).Code)
	assert.Equal(t, 404, doJSON(r, "GET", "/memories?db_id=nope", nil).Code)
	assert.Equal(t, 200, doJSON(r, "GET", "/memories?db_id=two", nil).Code)
}

func TestEvalRunExecutesPerformanceEval(t *testing.T) {
	_, r := newParityOS(t, AgentOSOptions{})
	w := doJSON(r, "POST", "/eval-runs", map[string]interface{}{"agent_id": "echo", "eval_type": "performance", "input": "ping", "num_iterations": 2, "name": "perf"})
	require.Equal(t, 200, w.Code, w.Body.String())
	var ev map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ev))
	id := ev["id"].(string)
	assert.Equal(t, "performance", ev["eval_type"])
	assert.Len(t, ev["eval_data"].(map[string]interface{})["run_times"], 2)

	w = doJSON(r, "PATCH", "/eval-runs/"+id, map[string]interface{}{"name": "renamed"})
	require.Equal(t, 200, w.Code)
	w = doJSON(r, "GET", "/eval-runs?eval_types=performance", nil)
	assert.Contains(t, w.Body.String(), "renamed")
	assert.Equal(t, 204, doJSON(r, "DELETE", "/eval-runs", map[string]interface{}{"eval_run_ids": []string{id}}).Code)
	assert.Equal(t, 404, doJSON(r, "GET", "/eval-runs/"+id, nil).Code)
}

func signHS256(t *testing.T, secret string, claims jwt.MapClaims) string {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return "Bearer " + tok
}

func TestJWTAuthorizationScopes(t *testing.T) {
	const secret = "test-secret"
	var seen *RunAuthorization
	o, _ := newParityOS(t, AgentOSOptions{JWT: &JWTConfig{
		VerificationKeys: []string{secret}, Algorithm: "HS256", Authorization: true, VerifyAudience: true,
	}})
	r := o.GetApp()
	r.GET("/whoami", func(c *gin.Context) {
		seen = RunAuthorizationFromContext(c.Request.Context())
		c.JSON(200, gin.H{"err": fmt.Sprint(RequirePermission(c.Request.Context(), "agents", "run", "echo"))})
	})
	exp := time.Now().Add(time.Hour).Unix()

	assert.Equal(t, 200, doJSON(r, "GET", "/health", nil).Code, "excluded route")
	assert.Equal(t, 401, doJSON(r, "GET", "/agents", nil).Code)
	assert.Equal(t, 401, doJSON(r, "GET", "/agents", nil, "Authorization", signHS256(t, "wrong", jwt.MapClaims{"sub": "u", "aud": "parity-os", "exp": exp})).Code)
	assert.Equal(t, 401, doJSON(r, "GET", "/agents", nil, "Authorization", signHS256(t, secret, jwt.MapClaims{"sub": "u", "aud": "other", "exp": exp})).Code)
	expired := doJSON(r, "GET", "/agents", nil, "Authorization", signHS256(t, secret, jwt.MapClaims{"sub": "u", "aud": "parity-os", "exp": time.Now().Add(-time.Hour).Unix()}))
	assert.Equal(t, 401, expired.Code)
	assert.Contains(t, expired.Body.String(), "Token has expired")

	// Per-resource scope: listing is filtered, other agents are forbidden.
	limited := signHS256(t, secret, jwt.MapClaims{"sub": "alice", "aud": "parity-os", "exp": exp, "scopes": []string{"agents:echo:read", "agents:echo:run"}})
	w := doJSON(r, "GET", "/agents", nil, "Authorization", limited)
	require.Equal(t, 200, w.Code)
	var agents []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &agents))
	require.Len(t, agents, 1)
	assert.Equal(t, "echo", agents[0]["name"])
	assert.Equal(t, 403, doJSON(r, "GET", "/agents/slow", nil, "Authorization", limited).Code)
	assert.Equal(t, 403, doJSON(r, "GET", "/sessions", nil, "Authorization", limited).Code)
	w = doJSON(r, "POST", "/agents/echo/runs", map[string]interface{}{"message": "hey", "stream": false}, "Authorization", limited)
	require.Equal(t, 200, w.Code, w.Body.String())

	// The authority reaches the request context.
	w = doJSON(r, "GET", "/whoami", nil, "Authorization", limited)
	require.Equal(t, 200, w.Code)
	require.NotNil(t, seen)
	assert.Equal(t, "alice", seen.Context.UserID)
	assert.JSONEq(t, `{"err":"<nil>"}`, w.Body.String())
	assert.Error(t, seen.Require("agents", "run", "slow"))

	admin := signHS256(t, secret, jwt.MapClaims{"sub": "root", "aud": "parity-os", "exp": exp, "scopes": "agent_os:admin"})
	assert.Equal(t, 200, doJSON(r, "GET", "/sessions", nil, "Authorization", admin).Code)
	// Legacy system:* alias for config:*.
	legacy := signHS256(t, secret, jwt.MapClaims{"sub": "ops", "aud": "parity-os", "exp": exp, "scopes": []string{"system:read"}})
	assert.Equal(t, 200, doJSON(r, "GET", "/config", nil, "Authorization", legacy).Code)
}

func TestSecurityKeyProtectsRoutes(t *testing.T) {
	_, r := newParityOS(t, AgentOSOptions{Settings: &AgentOSSettings{EnableCORS: true, SecurityKey: "k"}})
	assert.Equal(t, 200, doJSON(r, "GET", "/health", nil).Code)
	assert.Equal(t, 401, doJSON(r, "GET", "/sessions", nil).Code)
	assert.Equal(t, 200, doJSON(r, "GET", "/sessions", nil, "Authorization", "Bearer k").Code)
}

func TestScopeHelpers(t *testing.T) {
	assert.True(t, HasRequiredScopes([]string{"agents:*:run"}, []string{"agents:run"}, "agents", []string{"x"}, ""))
	assert.False(t, HasRequiredScopes([]string{"agents:a:read"}, []string{"sessions:read"}, "agents", []string{"a"}, ""))
	assert.Equal(t, []string{"agents:run"}, RequiredScopesForRoute(DefaultScopeMappings(), "POST", "/agents/x/runs"))
	assert.Equal(t, []string{"agents:read"}, RequiredScopesForRoute(DefaultScopeMappings(), "GET", "/agents/x/runs/y"))
	typ, id := ResourceContextFromPath("/teams/t1/runs")
	assert.Equal(t, "teams", typ)
	assert.Equal(t, "t1", id)
}

func TestCORSOriginPolicy(t *testing.T) {
	_, r := newParityOS(t, AgentOSOptions{Settings: &AgentOSSettings{EnableCORS: true, CORSOriginPatterns: []string{`https://[a-z]+\.example\.com`}}})
	w := doJSON(r, "GET", "/health", nil, "Origin", "https://app.example.com")
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	w = doJSON(r, "OPTIONS", "/agents", nil, "Origin", "https://evil.com", "Access-Control-Request-Method", "POST")
	assert.Equal(t, 400, w.Code)
}

func TestDiagnosticsEndpoint(t *testing.T) {
	t.Setenv("PARITY_SET", "1")
	_, r := newParityOS(t, AgentOSOptions{})
	w := doJSON(r, "GET", "/diagnostics?required_env=PARITY_SET,PARITY_MISSING_VAR", nil)
	require.Equal(t, 200, w.Code)
	var rep DiagnosticReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rep))
	assert.Equal(t, DiagnosticFail, rep.Status)
	byName := map[string]CheckResult{}
	for _, c := range rep.Checks {
		byName[c.Name] = c
	}
	assert.Equal(t, DiagnosticPass, byName["database"].Status)
	assert.Equal(t, "live", byName["database"].Evidence)
	assert.Equal(t, DiagnosticWarn, byName["authorization"].Status)
	assert.Equal(t, DiagnosticPass, byName["env:PARITY_SET"].Status)
	assert.Equal(t, DiagnosticFail, byName["env:PARITY_MISSING_VAR"].Status)
	assert.NotContains(t, w.Body.String(), `"1"`)
}

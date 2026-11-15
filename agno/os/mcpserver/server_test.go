package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func echoAgent(id string) *component.Func {
	return &component.Func{
		IDValue: id, NameValue: id, DescriptionValue: "echoes", KindValue: component.KindAgent,
		RunFunc: func(_ context.Context, req component.RunRequest) (*component.RunResult, error) {
			if req.Input == "pause" {
				return &component.RunResult{Status: component.StatusPaused,
					Requirements: []map[string]any{{"id": "r1", "tool_execution": map[string]any{"tool_call_id": "c1", "tool_name": "delete"}}}}, nil
			}
			return &component.RunResult{Content: "echo:" + req.Input + ":" + req.UserID}, nil
		},
		ContinueFunc: func(_ context.Context, req component.ContinueRequest) (*component.RunResult, error) {
			return &component.RunResult{RunID: req.RunID, SessionID: req.SessionID, Status: component.StatusCompleted,
				Content: "continued:" + req.Requirements[0]["id"].(string)}, nil
		},
		CancelFunc: func(context.Context, string) (bool, error) { return true, nil },
	}
}

func connect(t *testing.T, url string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func toolNames(t *testing.T, cs *mcp.ClientSession) []string {
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	return names
}

func text(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	return res.Content[0].(*mcp.TextContent).Text
}

func TestDefaultSurfaceOverTransport(t *testing.T) {
	reg := component.NewRegistry(echoAgent("a1"))
	srv, err := New(reg, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	cs := connect(t, ts.URL+"/mcp")

	names := strings.Join(toolNames(t, cs), ",")
	for _, want := range []string{"get_agentos_config", "run_agent", "run_team", "run_workflow", "continue_run", "cancel_run", "get_sessions", "get_session_runs"} {
		if !strings.Contains(names, want) {
			t.Errorf("missing tool %s in %s", want, names)
		}
	}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "run_agent", Arguments: map[string]any{"agent_id": "a1", "message": "hi", "user_id": "u1"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || text(res) != "echo:hi:u1" {
		t.Fatalf("unexpected result %+v %q", res, text(res))
	}
	sc := res.StructuredContent.(map[string]any)
	if sc["agent_id"] != "a1" || sc["status"] != "COMPLETED" || sc["session_id"] == "" {
		t.Fatalf("structured: %v", sc)
	}

	// Paused run carries requirements; continue_run resumes it.
	res, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "run_agent", Arguments: map[string]any{"agent_id": "a1", "message": "pause"}})
	sc = res.StructuredContent.(map[string]any)
	if sc["status"] != "PAUSED" || sc["requirements"] == nil {
		t.Fatalf("paused structured: %v", sc)
	}
	res, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "continue_run", Arguments: map[string]any{
		"run_id": sc["run_id"], "session_id": sc["session_id"], "agent_id": "a1", "requirements": sc["requirements"]}})
	if text(res) != "continued:r1" {
		t.Fatalf("continue: %q", text(res))
	}

	// Sessions without a store report an error result.
	res, _ = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_sessions", Arguments: map[string]any{}})
	if !res.IsError {
		t.Fatal("expected error without session store")
	}
}

func TestBoundedArguments(t *testing.T) {
	srv, _ := New(component.NewRegistry(echoAgent("a1")), &Config{DefaultTools: true, MaxMessageLength: 5})
	_, err := srv.CallTool(context.Background(), "run_agent", map[string]any{"agent_id": "a1", "message": "too long message"})
	if !errors.Is(err, errInvalidArgs) {
		t.Fatalf("want invalid args, got %v", err)
	}
	_, err = srv.CallTool(context.Background(), "run_agent", map[string]any{"agent_id": "a1", "message": "ok", "extra": 1})
	if !errors.Is(err, errInvalidArgs) {
		t.Fatalf("unknown property should be rejected, got %v", err)
	}
	_, err = srv.CallTool(context.Background(), "get_sessions", map[string]any{"limit": 1000})
	if !errors.Is(err, errInvalidArgs) {
		t.Fatalf("limit bound, got %v", err)
	}
}

func TestExposureAndLifecycle(t *testing.T) {
	a1, a2 := echoAgent("a1"), echoAgent("a2")
	reg := component.NewRegistry(a1, a2)
	srv, err := New(reg, &Config{Expose: []Exposure{{Component: a1, Name: "chief"}}, LifecycleTools: true})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range srv.Tools() {
		names = append(names, tl.Name)
	}
	if strings.Join(names, ",") != "continue_run,cancel_run,chief" {
		t.Fatalf("tools: %v", names)
	}
	res, _ := srv.CallTool(context.Background(), "chief", map[string]any{"message": "x"})
	if text(res) != "echo:x:" {
		t.Fatalf("chief: %q", text(res))
	}
	res, _ = srv.CallTool(context.Background(), "cancel_run", map[string]any{"run_id": "r", "agent_id": "a2"})
	if !res.IsError {
		t.Fatal("lifecycle must be bounded to exposed components")
	}
	res, _ = srv.CallTool(context.Background(), "cancel_run", map[string]any{"run_id": "r", "agent_id": "a1"})
	if res.IsError {
		t.Fatalf("cancel exposed: %q", text(res))
	}
	// Collisions and invalid names fail fast.
	if _, err := New(reg, &Config{DefaultTools: true, Expose: []Exposure{{Component: a1, Name: "run_agent"}}}); !errors.Is(err, errNameCollision) {
		t.Fatalf("want collision, got %v", err)
	}
	if _, err := New(reg, &Config{Expose: []Exposure{{Component: a1, Name: "1bad name"}}}); !errors.Is(err, errInvalidToolNam) {
		t.Fatalf("want invalid name, got %v", err)
	}
	// Paused result without continue_run points to recovery.
	srv, _ = New(reg, &Config{Expose: []Exposure{{Component: a1}}})
	res, _ = srv.CallTool(context.Background(), "a1", map[string]any{"message": "pause"})
	if !strings.Contains(text(res), "continue_run tool is not registered") {
		t.Fatalf("hint missing: %q", text(res))
	}
}

func TestHTTPSurface(t *testing.T) {
	srv, _ := New(component.NewRegistry(echoAgent("a1")), &Config{DefaultTools: true, Name: "My OS", Path: "/tools/mcp", PathAliases: []string{"/legacy"}})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp, err := noRedirect.Get(ts.URL + "/tools/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/tools/mcp/server-card" {
		t.Fatalf("browser redirect: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}

	resp, _ = http.Get(ts.URL + "/legacy/server-card")
	var card map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&card)
	resp.Body.Close()
	if card["name"] != "localhost/my-os" || len(card["tools"].([]any)) != 8 {
		t.Fatalf("card: %v", card)
	}
	remote := card["remotes"].([]any)[0].(map[string]any)
	if !strings.HasSuffix(remote["url"].(string), "/legacy") {
		t.Fatalf("card url: %v", remote)
	}

	post := func(path, body string) (int, map[string]any) {
		resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return resp.StatusCode, m
	}
	code, m := post("/tools/mcp/server/tools/run_agent/run", `{"arguments":{"agent_id":"a1","message":"hey"}}`)
	if code != 200 || m["isError"] != false {
		t.Fatalf("tool run: %d %v", code, m)
	}
	if code, m = post("/tools/mcp/server/tools/run_agent/run", `{"arguments":{"agent_id":"a1"}}`); code != 400 || m["error"] != "invalid_arguments" {
		t.Fatalf("invalid args: %d %v", code, m)
	}
	if code, _ = post("/tools/mcp/server/tools/nope/run", `{}`); code != 404 {
		t.Fatalf("unknown tool: %d", code)
	}

	// DNS-rebinding guard: an open server only answers localhost hosts.
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/tools/mcp", strings.NewReader("{}"))
	req.Host = "evil.example.com"
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("host check: %d", resp.StatusCode)
	}
}

func TestAuthAndIdentity(t *testing.T) {
	cfg := &Config{
		DefaultTools: true,
		Authenticate: func(r *http.Request) (*Identity, error) {
			tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tok == "" {
				return nil, errors.New("missing token")
			}
			return &Identity{UserID: tok}, nil
		},
		Authorize: func(userID string) bool { return userID != "mallory" },
	}
	srv, _ := New(component.NewRegistry(echoAgent("a1")), cfg)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	call := func(token string) (int, map[string]any) {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp/server/tools/run_agent/run",
			strings.NewReader(`{"arguments":{"agent_id":"a1","message":"m","user_id":"spoofed"}}`))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set(hdrUser, "spoofed-header")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return resp.StatusCode, m
	}
	if code, _ := call(""); code != 401 {
		t.Fatalf("no token: %d", code)
	}
	if code, _ := call("mallory"); code != 401 {
		t.Fatalf("authorize gate: %d", code)
	}
	code, m := call("alice")
	if code != 200 {
		t.Fatalf("alice: %d %v", code, m)
	}
	content := m["content"].([]any)[0].(map[string]any)["text"]
	if content != "echo:m:alice" {
		t.Fatalf("identity must replace advisory user_id: %v", content)
	}

	// Over the transport too.
	httpClient := &http.Client{Transport: roundTripper(func(r *http.Request) (*http.Response, error) {
		r.Header.Set("Authorization", "Bearer bob")
		return http.DefaultTransport.RoundTrip(r)
	})}
	client := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", HTTPClient: httpClient}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "run_agent", Arguments: map[string]any{"agent_id": "a1", "message": "m"}})
	if err != nil || text(res) != "echo:m:bob" {
		t.Fatalf("transport identity: %v %q", err, text(res))
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConfigValidation(t *testing.T) {
	bad := []*Config{
		{Path: "/"},
		{Path: "/mcp", PathAliases: []string{"/mcp/server"}},
		{RootHost: "https://x"},
		{ResultMode: "weird"},
		{IncludeTags: []string{"nope"}},
	}
	for i, c := range bad {
		if _, err := New(nil, c); err == nil {
			t.Errorf("config %d should fail", i)
		}
	}
}

func TestRootHost(t *testing.T) {
	srv, _ := New(component.NewRegistry(echoAgent("a1")), &Config{DefaultTools: true, RootHost: "mcp.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/server-card", nil)
	req.Host = "mcp.example.com"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "com.example.mcp/agentos") {
		t.Fatalf("root host card: %d %s", rec.Code, rec.Body.String())
	}
}

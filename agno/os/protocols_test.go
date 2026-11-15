package os

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/os/a2aserver"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/mcpserver"
)

func TestMountProtocols(t *testing.T) {
	echo := &component.Func{IDValue: "echo", KindValue: component.KindAgent,
		RunFunc: func(_ context.Context, r component.RunRequest) (*component.RunResult, error) {
			return &component.RunResult{Content: "echo " + r.Input}, nil
		}}
	aos, err := NewAgentOS(AgentOSOptions{OSID: "test-os", Protocols: ProtocolOptions{
		MCP:   &mcpserver.Config{DefaultTools: true},
		A2A:   &a2aserver.Config{},
		AGUI:  &AGUIOptions{},
		Extra: []component.Component{echo},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(aos.GetApp())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/mcp/server-card")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("mcp card: %v %v", err, resp)
	}
	resp, _ = http.Post(ts.URL+"/mcp/server/tools/run_agent/run", "application/json", strings.NewReader(`{"arguments":{"agent_id":"echo","message":"hi"}}`))
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["isError"] != false {
		t.Fatalf("mcp run: %v", out)
	}
	resp, _ = http.Get(ts.URL + "/a2a/agents/echo/.well-known/agent-card.json")
	if resp.StatusCode != 200 {
		t.Fatalf("a2a card: %d", resp.StatusCode)
	}
	resp, _ = http.Post(ts.URL+"/agui", "application/json", strings.NewReader(`{"threadId":"t","messages":[{"id":"1","role":"user","content":"yo"}]}`))
	buf := new(strings.Builder)
	_, _ = bufCopy(buf, resp)
	if !strings.Contains(buf.String(), "RUN_FINISHED") || !strings.Contains(buf.String(), "echo yo") {
		t.Fatalf("agui: %s", buf.String())
	}
	// Existing routes still work.
	if resp, _ := http.Get(ts.URL + "/health"); resp.StatusCode != 200 {
		t.Fatalf("health: %d", resp.StatusCode)
	}
}

func bufCopy(sb *strings.Builder, resp *http.Response) (int, error) {
	defer resp.Body.Close()
	b := make([]byte, 64*1024)
	n, err := resp.Body.Read(b)
	total := 0
	for n > 0 {
		sb.Write(b[:n])
		total += n
		if err != nil {
			break
		}
		n, err = resp.Body.Read(b)
	}
	return total, nil
}

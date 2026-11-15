package os

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/os/internal/hitltest"
)

// TestContinueAgentRunEndpoint drives a real paused run through the REST
// continue endpoint: run -> PAUSED with a requirement -> continue with the
// tool confirmed -> COMPLETED with the tool actually executed.
func TestContinueAgentRunEndpoint(t *testing.T) {
	ag, files, err := hitltest.NewAgent("cleaner")
	require.NoError(t, err)

	aos, err := NewAgentOS(AgentOSOptions{OSID: "hitl-os", Agents: []*agent.Agent{ag}})
	require.NoError(t, err)
	router := aos.GetApp()

	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		router.ServeHTTP(w, req)
		return w
	}

	// 1. Start a run directly on the agent so we get its paused response.
	paused, err := ag.Run("delete /tmp/report.txt", agent.WithSessionID("s1"))
	require.NoError(t, err)
	require.True(t, paused.IsPaused(), "run should pause on the confirmation-gated tool")
	reqs := paused.ActiveRequirements()
	require.Len(t, reqs, 1)
	assert.Empty(t, files.Deleted(), "tool must not run before confirmation")

	// 2. Continue through REST with the tool confirmed (Python sends the
	//    ToolExecution dicts in the "tools" form field).
	tool := map[string]any{
		"tool_call_id": reqs[0].ToolExecution.ToolCallID,
		"tool_name":    reqs[0].ToolExecution.ToolName,
		"tool_args":    reqs[0].ToolExecution.ToolArgs,
		"confirmed":    true,
	}
	toolsJSON, _ := json.Marshal([]map[string]any{tool})
	w := post("/agents/cleaner/runs/"+paused.RunID+"/continue", url.Values{
		"tools":      {string(toolsJSON)},
		"session_id": {"s1"},
		"stream":     {"false"},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "COMPLETED", out["status"])
	assert.Contains(t, out["content"], "final:")
	// The stub model always targets the same path; what matters is that the
	// confirmed tool ran exactly once, and only after the continue call.
	assert.Len(t, files.Deleted(), 1)

	// 3. Continuing an unknown run is a 404, not a fresh run.
	w = post("/agents/cleaner/runs/run_does_not_exist/continue", url.Values{"stream": {"false"}})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

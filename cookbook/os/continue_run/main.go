// AgentOS: continue a paused (human-in-the-loop) run over REST.
//
// POST /agents/{id}/runs/{run_id}/continue resumes a PAUSED run, as in the
// Python AgentOS. The "tools" form field is a JSON list of the paused tool
// executions carrying the user's decision (confirmed / confirmation_note,
// user input values, or result for external execution). stream defaults to
// true (SSE); pass stream=false for a JSON response.
//
// This example runs offline: it serves AgentOS in-process with a scripted stub
// model and a confirmation-gated tool, pauses a run, then continues it through
// the HTTP endpoint. Against a real server the call looks like:
//
//	curl -X POST http://localhost:7777/agents/cleaner/runs/<run_id>/continue \
//	  -F session_id=s1 -F stream=false \
//	  -F 'tools=[{"tool_call_id":"<id>","tool_name":"Files_Delete","tool_args":{"path":"/tmp/x"},"confirmed":true}]'
//
// No env vars or services needed.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/devalexandre/agno-golang/agno/agent"
	agnoos "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type DeleteParams struct {
	Path string `json:"path" description:"File to delete" required:"true"`
}

type FileTool struct{ toolkit.Toolkit }

func (f *FileTool) Delete(p DeleteParams) (interface{}, error) {
	return "deleted " + p.Path, nil
}

func main() {
	files := &FileTool{Toolkit: toolkit.NewToolkit()}
	files.Name, files.Description = "Files", "File operations"
	files.RegisterWithOptions("Delete", "Delete a file", files, files.Delete, DeleteParams{}, toolkit.WithConfirmation())

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "cleaner",
		Model:   toolcallstub.New("Done.", "Files_Delete", `{"path":"/tmp/x"}`),
		Tools:   []toolkit.Tool{files},
	})
	if err != nil {
		log.Fatal(err)
	}

	aos, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{OSID: "hitl-os", Agents: []*agent.Agent{ag}})
	if err != nil {
		log.Fatal(err)
	}
	srv := httptest.NewServer(aos.GetApp())
	defer srv.Close()

	// 1. The run pauses on the confirmation-gated tool.
	paused, err := ag.Run("delete /tmp/x", agent.WithSessionID("s1"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("status:", paused.Status)
	req := paused.ActiveRequirements()[0]
	fmt.Printf("waiting for confirmation of %s(%v)\n", req.ToolExecution.ToolName, req.ToolExecution.ToolArgs)

	// 2. A client (UI, another service) confirms it through REST.
	tools, _ := json.Marshal([]map[string]any{{
		"tool_call_id": req.ToolExecution.ToolCallID,
		"tool_name":    req.ToolExecution.ToolName,
		"tool_args":    req.ToolExecution.ToolArgs,
		"confirmed":    true,
	}})
	resp, err := http.PostForm(srv.URL+"/agents/cleaner/runs/"+paused.RunID+"/continue", url.Values{
		"tools":      {string(tools)},
		"session_id": {"s1"},
		"stream":     {"false"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		log.Fatalf("unexpected response %d: %s", resp.StatusCode, body)
	}
	fmt.Println("continue HTTP status:", resp.StatusCode)
	fmt.Println("run status:", out["status"])
	fmt.Println("content:", out["content"])
}

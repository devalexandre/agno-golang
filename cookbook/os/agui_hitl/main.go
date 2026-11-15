// AG-UI human-in-the-loop: pause an agent on a confirmation-gated tool and
// resume it from the frontend.
//
// What it shows
//   - a tool registered with toolkit.WithConfirmation pauses the agent run;
//     over AG-UI the run ends with TOOL_CALL_START/ARGS/END for the pending
//     call, then RUN_FINISHED
//   - the frontend answers with a trailing "tool" message on the same thread
//     whose content follows the pause type:
//     confirmation        {"accepted": true|false, "note": "..."}
//     user_input          {"values": {"field": value}}
//     user_feedback       {"selections": {"question": ["label"]}}
//     external_execution  the raw result
//   - the answer is applied with RunRequirement.Confirm/Reject/... and the run
//     resumes through Agent.ContinueRun
//
// Runs offline: a scripted stub model and an in-process HTTP server.
//
//	go run ./cookbook/os/agui_hitl
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/os/agui"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type DeleteParams struct {
	Path string `json:"path" description:"File to delete" required:"true"`
}

type Files struct{ toolkit.Toolkit }

func (f *Files) Delete(p DeleteParams) (interface{}, error) { return "deleted " + p.Path, nil }

// post sends a RunAgentInput and prints the AG-UI events; it returns the id
// of the last tool call it saw.
func post(url string, in agui.RunAgentInput) string {
	b, _ := json.Marshal(in)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(b)))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	var callID string
	_ = a2a.ReadSSE(resp.Body, func(data []byte) error {
		var ev map[string]any
		_ = json.Unmarshal(data, &ev)
		switch ev["type"] {
		case agui.ToolCallStart:
			callID, _ = ev["toolCallId"].(string)
			fmt.Printf("  %s %v\n", ev["type"], ev["toolCallName"])
		case agui.ToolCallArgs, agui.TextMessageContent:
			fmt.Printf("  %s %v\n", ev["type"], ev["delta"])
		case agui.RunError:
			fmt.Printf("  %s %v\n", ev["type"], ev["message"])
		default:
			fmt.Printf("  %s\n", ev["type"])
		}
		return nil
	})
	return callID
}

func text(s string) json.RawMessage { b, _ := json.Marshal(s); return b }

func main() {
	files := &Files{Toolkit: toolkit.NewToolkit()}
	files.Name, files.Description = "Files", "File operations"
	files.RegisterWithOptions("Delete", "Delete a file", files, files.Delete, DeleteParams{}, toolkit.WithConfirmation())

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: context.Background(),
		Name:    "Cleaner",
		Model:   toolcallstub.New("Done.", "Files_Delete", `{"path":"/tmp/report.csv"}`),
		Tools:   []toolkit.Tool{files},
	})
	if err != nil {
		log.Fatal(err)
	}
	srv, err := agui.New(component.FromAgent(ag), nil)
	if err != nil {
		log.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	user := agui.Message{ID: "1", Role: "user", Content: text("Delete /tmp/report.csv")}

	fmt.Println("1. run: the agent pauses on Files_Delete")
	callID := post(ts.URL+"/agui", agui.RunAgentInput{ThreadID: "t1", RunID: "r1", Messages: []agui.Message{user}})

	fmt.Println("2. resume: the frontend confirms the pending call")
	answer := agui.Message{ID: "2", Role: "tool", ToolCallID: callID, Content: text(`{"accepted": true}`)}
	post(ts.URL+"/agui", agui.RunAgentInput{ThreadID: "t1", RunID: "r2", Messages: []agui.Message{user, answer}})
}

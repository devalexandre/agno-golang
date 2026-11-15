package agui

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/internal/hitltest"
)

// A real agent pauses on a confirmation-gated tool, the frontend sees the
// pending tool call, and its tool message resumes the run via Agent.ContinueRun.
func TestRealAgentPauseAndResume(t *testing.T) {
	for _, tc := range []struct {
		name, answer, want string
		deleted            int
	}{
		{"accept", `{"accepted": true}`, "final: deleted /tmp/x", 1},
		{"reject", `{"accepted": false, "note": "not that file"}`, "final: not that file", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ag, files, err := hitltest.NewAgent("cleaner")
			if err != nil {
				t.Fatal(err)
			}
			srv, _ := New(component.FromAgent(ag), nil)
			ts := httptest.NewServer(srv)
			defer ts.Close()

			evs := post(t, ts.URL+"/agui", RunAgentInput{ThreadID: "th", Messages: []Message{userMsg("delete /tmp/x")}})
			var callID string
			for _, e := range evs {
				if e["type"] == ToolCallStart && e["toolCallName"] == hitltest.ToolName {
					callID, _ = e["toolCallId"].(string)
				}
				if e["type"] == ToolCallArgs && !strings.Contains(e["delta"].(string), "/tmp/x") {
					t.Fatalf("tool args: %v", e)
				}
			}
			if callID == "" || !strings.HasSuffix(types(evs), "TOOL_CALL_END,RUN_FINISHED") {
				t.Fatalf("paused run must surface the pending tool call: %s", types(evs))
			}
			if len(files.Deleted()) != 0 {
				t.Fatal("tool ran before confirmation")
			}

			answer, _ := json.Marshal(tc.answer)
			evs = post(t, ts.URL+"/agui", RunAgentInput{ThreadID: "th", Messages: []Message{
				userMsg("delete /tmp/x"), {ID: "t", Role: "tool", ToolCallID: callID, Content: answer}}})
			var text string
			for _, e := range evs {
				if e["type"] == TextMessageContent {
					text += e["delta"].(string)
				}
			}
			if text != tc.want || len(files.Deleted()) != tc.deleted {
				t.Fatalf("resume: %s %q deleted=%v", types(evs), text, files.Deleted())
			}
		})
	}
}

// Human-in-the-loop: tool confirmation.
//
// A method registered with toolkit.WithConfirmation() pauses the run when the
// model calls it. Run returns Status=PAUSED with one Requirement per paused
// call; the caller confirms or rejects it and resumes with ContinueRun. With a
// DB configured the paused state is stored in the session, so a new process
// (a new Agent with the same DB and SessionID) can continue the run.
//
// Runs offline with a scripted stub model; no env vars or services needed.
// Replace toolcallstub with a real model (e.g. openai/chat + OPENAI_API_KEY)
// to try it against an LLM.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	inmemory "github.com/devalexandre/agno-golang/agno/storage/in_memory"
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

func newFileTool() *FileTool {
	t := &FileTool{Toolkit: toolkit.NewToolkit()}
	t.Name, t.Description = "Files", "File operations"
	t.RegisterWithOptions("Delete", "Delete a file", t, t.Delete, DeleteParams{}, toolkit.WithConfirmation())
	return t
}

func newAgent(db *inmemory.InMemoryDB) *agent.Agent {
	a, err := agent.NewAgent(agent.AgentConfig{
		Name:      "file-assistant",
		Model:     toolcallstub.New("Done.", "Files_Delete", `{"path":"/tmp/report.txt"}`),
		Tools:     []toolkit.Tool{newFileTool()},
		DB:        db,
		SessionID: "hitl-demo-session",
	})
	if err != nil {
		log.Fatal(err)
	}
	return a
}

func main() {
	ctx := context.Background()
	db := inmemory.New()

	// 1. Confirm in the same process.
	a := newAgent(db)
	resp, err := a.Run("Delete /tmp/report.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s\n%s\n", resp.Status, resp.TextContent)
	for _, req := range resp.ActiveRequirements() {
		fmt.Printf("-> confirming %s %v\n", req.ToolExecution.ToolName, req.ToolExecution.ToolArgs)
		_ = req.Confirm()
	}
	out, err := a.ContinueRun(ctx, &resp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s content=%q\n\n", out.Status, out.TextContent)

	// 2. Reject, continuing from a fresh agent (simulates a process restart).
	resp, _ = newAgent(db).Run("Delete /tmp/report.txt")
	runID := resp.RunID
	restarted := newAgent(db)
	paused, err := restarted.PausedRun(runID) // loaded from the session
	if err != nil {
		log.Fatal(err)
	}
	_ = paused.Requirements[0].Reject("the user wants to keep this file")
	out, err = restarted.ContinueRun(ctx, runID, paused.Requirements...)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("status=%s content=%q\n", out.Status, out.TextContent)
}

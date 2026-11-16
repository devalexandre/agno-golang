// Tool-result offloading (agno/offload).
//
// A tool returns a very large result. With AgentConfig.OffloadToolResults set,
// the agent never puts that payload in the transcript: it is stored in a
// per-session filesystem namespace and the model sees a short envelope (head
// and tail preview, size, result id). The model then reads focused passages
// back with search_result (regex) and read_result (paged).
//
// Runs fully offline with a stub model and an in-memory store. Requires no
// env vars or services. Use offload.NewResultStore(fs.NewLocalBackend(dir))
// to keep payloads on disk instead.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/offload"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

type stubModel struct{}

func (stubModel) Invoke(context.Context, []models.Message, ...models.Option) (*models.MessageResponse, error) {
	return &models.MessageResponse{Role: "assistant", Content: "done"}, nil
}
func (stubModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }
func (stubModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) GetID() string { return "stub" }

// logTool returns thousands of log lines.
type logTool struct{ toolkit.Toolkit }

type logParams struct {
	Service string `json:"service" description:"Service name" required:"true"`
}

func (l *logTool) Fetch(p logParams) (interface{}, error) {
	var b strings.Builder
	for i := 1; i <= 3000; i++ {
		level := "INFO"
		if i%997 == 0 {
			level = "ERROR"
		}
		fmt.Fprintf(&b, "2026-11-14T10:%02d:%02d %s %s request=%d latency_ms=%d\n", (i/60)%60, i%60, level, p.Service, i, 20+i%50)
	}
	b.WriteString("SUMMARY: 3000 requests, 3 errors")
	return b.String(), nil
}

func main() {
	lt := &logTool{Toolkit: toolkit.NewToolkit()}
	lt.Name = "logs"
	lt.Description = "Fetch service logs"
	lt.Register("fetch", "Fetch the latest logs of a service", lt, lt.Fetch, logParams{})

	store := offload.NewMemoryResultStore(offload.WithThresholdChars(4000))

	a, err := agent.NewAgent(agent.AgentConfig{
		Context:            context.Background(),
		Model:              stubModel{},
		SessionID:          "demo-session",
		Tools:              []toolkit.Tool{lt},
		OffloadToolResults: store,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Simulate the model calling the tool: the agent's wrapped tool returns an envelope.
	var envelope string
	for _, t := range a.GetTools() {
		if _, ok := t.GetMethods()["logs_fetch"]; ok {
			out, _ := t.Execute("logs_fetch", json.RawMessage(`{"service":"checkout"}`))
			envelope = out.(string)
		}
	}
	fmt.Printf("What the model sees (%d chars instead of ~200KB):\n%s\n\n", len(envelope), envelope)

	id := regexp.MustCompile(`id="(res_[0-9a-f]+)"`).FindStringSubmatch(envelope)[1]

	// The model follows up with the read-back tools.
	for _, t := range a.GetTools() {
		if _, ok := t.GetMethods()["offload_search_result"]; !ok {
			continue
		}
		out, _ := t.Execute("offload_search_result", json.RawMessage(fmt.Sprintf(`{"result_id":%q,"pattern":"ERROR","context_lines":1}`, id)))
		fmt.Printf("search_result:\n%s\n\n", out)
		out, _ = t.Execute("offload_read_result", json.RawMessage(fmt.Sprintf(`{"result_id":%q,"start_line":2990,"end_line":2992}`, id)))
		fmt.Printf("read_result:\n%s\n", out)
	}

	refs, _ := store.LiveIDs("demo-session", 10)
	fmt.Printf("\nstored results in session: %d\n", len(refs))
}

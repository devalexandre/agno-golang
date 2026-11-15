package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/fs"
	"github.com/devalexandre/agno-golang/agno/metrics"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/offload"
)

func TestFrameworkModulesWiring(t *testing.T) {
	fm := &fakeModel{handler: func(_ context.Context, msgs []models.Message) (*models.MessageResponse, error) {
		return &models.MessageResponse{Role: "assistant", Content: "ok",
			Metrics: &metrics.MessageMetrics{Tokens: metrics.Tokens{InputTokens: 7, OutputTokens: 3, TotalTokens: 10}}}, nil
	}}
	notes := fs.MustNew(fs.NewMemoryBackend(), fs.WithNamespace("notes"))
	shared := fs.MustNew(notes.Backend(), fs.WithNamespace("shared"), fs.WithReadOnly())
	store := offload.NewMemoryResultStore(offload.WithThresholdChars(50))

	a, err := NewAgent(AgentConfig{
		Context:            context.Background(),
		Model:              fm,
		Instructions:       "be brief",
		FileSystems:        []*fs.FileSystem{notes, shared},
		OffloadToolResults: store,
	})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range a.GetTools() {
		for m := range tool.GetMethods() {
			names[m] = true
		}
	}
	for _, want := range []string{"filesystem_write_file", "filesystem_shared_read_file", "offload_read_result"} {
		if !names[want] {
			t.Fatalf("missing tool %s in %v", want, names)
		}
	}
	if names["filesystem_shared_write_file"] {
		t.Fatal("read-only store must not expose write tools")
	}
	if !strings.Contains(a.instructions, "be brief") || !strings.Contains(a.instructions, "durable filesystem") || !strings.Contains(a.instructions, "read_result") {
		t.Fatalf("instructions not composed: %s", a.instructions)
	}

	// Offloading applies to filesystem tool output too.
	for _, tool := range a.GetTools() {
		if _, ok := tool.GetMethods()["filesystem_write_file"]; ok {
			big := strings.Repeat("row\n", 100)
			out, _ := tool.Execute("filesystem_write_file", json.RawMessage(`{"path":"big.txt","content":`+jsonQuote(big)+`}`))
			if !strings.HasPrefix(out.(string), "Wrote") {
				t.Fatalf("write: %v", out)
			}
			out, _ = tool.Execute("filesystem_read_file", json.RawMessage(`{"path":"big.txt"}`))
			if !strings.Contains(out.(string), "<result id=\"res_") {
				t.Fatalf("large read should be offloaded: %.200s", out)
			}
		}
	}

	resp, err := a.Run("hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.RunMetrics == nil || resp.RunMetrics.TotalTokens < 10 {
		t.Fatalf("typed metrics missing: %+v", resp.RunMetrics)
	}
	if resp.Metrics["total_tokens"] == nil || resp.Metrics["details"] == nil {
		t.Fatalf("metrics map missing: %v", resp.Metrics)
	}
	if d := resp.RunMetrics.Snapshot().Details["model"]; len(d) != 1 || d[0].ID != "fake" {
		t.Fatalf("details %+v", d)
	}
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

package compression

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/metrics"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

type fakeModel struct{ calls int }

func (f *fakeModel) Invoke(ctx context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	f.calls++
	return &models.MessageResponse{Content: "SHORT", Metrics: &metrics.MessageMetrics{Tokens: metrics.Tokens{InputTokens: 10, OutputTokens: 2, TotalTokens: 12}}}, nil
}
func (f *fakeModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (f *fakeModel) InvokeStream(context.Context, []models.Message, ...models.Option) error {
	return nil
}
func (f *fakeModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (f *fakeModel) GetID() string { return "fake-mini" }

func id(s string) *string { return &s }

func TestShouldCompressAndCompress(t *testing.T) {
	fm := &fakeModel{}
	m := NewManager(fm)
	msgs := []models.Message{
		{Role: models.TypeUserRole, Content: "q"},
		{Role: models.TypeToolRole, Content: "long 1", ToolCallID: id("a")},
		{Role: models.TypeToolRole, Content: "long 2", ToolCallID: id("b")},
	}
	if m.ShouldCompress(msgs) {
		t.Fatal("2 < limit 3")
	}
	msgs = append(msgs, models.Message{Role: models.TypeToolRole, Content: "long 3", ToolCallID: id("c")})
	if !m.ShouldCompress(msgs) {
		t.Fatal("3 >= limit")
	}
	rm := metrics.NewRunMetrics()
	m.Compress(context.Background(), msgs, rm)
	if msgs[1].Content != "SHORT" || fm.calls != 3 || m.ShouldCompress(msgs) {
		t.Fatalf("compress failed: %+v calls=%d", msgs, fm.calls)
	}
	if rm.Snapshot().Details["compression_model"][0].TotalTokens != 36 {
		t.Fatalf("metrics %+v", rm.ToMap())
	}
	if m.Stats()["tool_results_compressed"] != 3 {
		t.Fatal("stats")
	}
}

type echo struct{ toolkit.Toolkit }
type p struct {
	S string `json:"s"`
}

func (e *echo) Run(x p) (interface{}, error) { return x.S, nil }

func TestWrap(t *testing.T) {
	m := NewManager(&fakeModel{})
	m.MinChars = 10
	e := &echo{Toolkit: toolkit.NewToolkit()}
	e.Name = "e"
	e.Register("run", "echo", e, e.Run, p{})
	w := m.Wrap(context.Background(), e, nil)
	out, _ := w.Execute("e_run", json.RawMessage(`{"s":"tiny"}`))
	if out != "tiny" {
		t.Fatal("small passes through")
	}
	out, _ = w.Execute("e_run", json.RawMessage(`{"s":"`+strings.Repeat("z", 50)+`"}`))
	if out != "SHORT" {
		t.Fatalf("got %v", out)
	}
}

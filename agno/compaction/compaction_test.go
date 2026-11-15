package compaction

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
)

type stubModel struct {
	reply string
	err   error
	calls [][]models.Message
}

func (s *stubModel) Invoke(_ context.Context, m []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	s.calls = append(s.calls, m)
	if s.err != nil {
		return nil, s.err
	}
	return &models.MessageResponse{Role: "assistant", Content: s.reply}, nil
}
func (s *stubModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (s *stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error {
	return nil
}
func (s *stubModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (s *stubModel) GetID() string { return "stub" }

func conversation(runs int, size int) []models.Message {
	var out []models.Message
	for i := 0; i < runs; i++ {
		out = append(out,
			models.Message{Role: models.TypeUserRole, Content: "question " + string(rune('a'+i)) + strings.Repeat(" q", size)},
			models.Message{Role: models.TypeAssistantRole, Content: "answer " + string(rune('a'+i)) + strings.Repeat(" r", size)},
		)
	}
	return out
}

func TestNormalizedDefaults(t *testing.T) {
	c, err := Config{}.Normalized()
	if err != nil {
		t.Fatal(err)
	}
	if c.CompactAtTokens != DefaultCompactAtTokens || c.UncompactedRuns != DefaultUncompactedRuns {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	o, _ := OverflowOnly().Normalized()
	if o.HasThreshold() {
		t.Fatal("OverflowOnly must not have a threshold")
	}
	if _, err := (Config{UncompactedRuns: 2, UncompactedTokens: 10}).Normalized(); err == nil {
		t.Fatal("expected mutual exclusion error")
	}
	if _, err := (Config{CompactAtTokens: 100, UncompactedTokens: 100}).Normalized(); err == nil {
		t.Fatal("expected tail >= threshold error")
	}
}

func TestEstimateTokensHeuristic(t *testing.T) {
	if got := EstimateTextTokens(strings.Repeat("a", 40)); got != 10 {
		t.Fatalf("want 10, got %d", got)
	}
	m := models.Message{Role: "assistant", Content: strings.Repeat("a", 39), ToolCalls: []tools.ToolCall{{Function: tools.FunctionCall{Arguments: "xx"}}}}
	if got := EstimateMessageTokens(m); got != 10 { // (39 + 1 + 2) / 4
		t.Fatalf("want 10, got %d", got)
	}
}

func TestChooseBoundaryKeepsToolBatchTogether(t *testing.T) {
	id := "call_1"
	msgs := []models.Message{
		{Role: models.TypeSystemRole, Content: "sys"},
		{Role: models.TypeUserRole, Content: "u1"},
		{Role: models.TypeAssistantRole, Content: "a1"},
		{Role: models.TypeUserRole, Content: "u2"},
		{Role: models.TypeAssistantRole, ToolCalls: []tools.ToolCall{{ID: id, Function: tools.FunctionCall{Name: "f"}}}},
		{Role: models.TypeToolRole, Content: "result", ToolCallID: &id},
		{Role: models.TypeAssistantRole, Content: "a2"},
	}
	// Asking to keep from the tool message must snap back to its batch head.
	if got := ChooseBoundary(msgs, 0, 5, 0); got != 4 {
		t.Fatalf("want boundary 4 (batch head), got %d", got)
	}
	// Nothing past the system prompt can be a boundary.
	if got := ChooseBoundary(msgs, 0, 1, 0); got != -1 {
		t.Fatalf("want -1, got %d", got)
	}
}

func TestCompactFoldsOlderRuns(t *testing.T) {
	model := &stubModel{reply: "## Goal\nhelp the user"}
	c, err := New(Config{Model: model, UncompactedRuns: 2})
	if err != nil {
		t.Fatal(err)
	}
	history := conversation(5, 10)
	res, err := c.Compact(context.Background(), history, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Compacted() {
		t.Fatalf("expected compaction, got %s: %s", res.Status, res.Message)
	}
	if res.Record.MessagesCompacted != 6 || len(res.Kept) != 4 {
		t.Fatalf("want 6 folded / 4 kept, got %d / %d", res.Record.MessagesCompacted, len(res.Kept))
	}
	if res.Record.FirstKeptMessageID != MessageFingerprint(history[6]) {
		t.Fatal("anchor must be the first kept message")
	}
	if len(model.calls) != 1 || !strings.Contains(model.calls[0][0].Content, "## Goal") {
		t.Fatal("summarizer must receive the compaction prompt")
	}
	if !strings.Contains(model.calls[0][1].Content, "## user") {
		t.Fatal("transcript must be rendered as markdown")
	}

	view := ApplyRecord(history, res.Record)
	if len(view) != 5 || !IsInjectedSummary(view[0]) {
		t.Fatalf("view must be summary + 4 kept messages, got %d", len(view))
	}

	// Folding again with the previous record chains summaries.
	more := append(append([]models.Message(nil), res.Kept...), conversation(3, 10)...)
	res2, err := c.Compact(context.Background(), more, Options{Previous: res.Record})
	if err != nil || !res2.Compacted() {
		t.Fatalf("second fold failed: %v %+v", err, res2)
	}
	if !strings.Contains(model.calls[1][1].Content, "Summary of the conversation before this point:\n## Goal") {
		t.Fatal("previous summary must be folded into the new one")
	}
	if st := c.Stats(); st.Compactions != 2 {
		t.Fatalf("want 2 compactions, got %d", st.Compactions)
	}
}

func TestCompactDeclines(t *testing.T) {
	model := &stubModel{reply: "s"}
	c, _ := New(Config{Model: model, UncompactedRuns: 5})
	res, _ := c.Compact(context.Background(), conversation(3, 1), Options{})
	if res.Status != StatusNothingToFold {
		t.Fatalf("want nothing_to_fold, got %s", res.Status)
	}
	res, _ = c.Compact(context.Background(), nil, Options{})
	if res.Status != StatusNoHistory {
		t.Fatalf("want no_history, got %s", res.Status)
	}
	failing := &stubModel{err: errors.New("boom")}
	c2, _ := New(Config{Model: failing, UncompactedRuns: 1})
	res, err := c2.Compact(context.Background(), conversation(3, 1), Options{})
	if err != nil || res.Status != StatusSummaryFailed {
		t.Fatalf("want summary_failed, got %v %s", err, res.Status)
	}
	ratio, _ := New(Config{Model: model, UncompactedRuns: 1, EnforceMinFoldRatio: true, MinFoldRatio: 100})
	res, _ = ratio.Compact(context.Background(), conversation(3, 10), Options{})
	if res.Status != StatusNotWorthIt {
		t.Fatalf("want not_worth_it, got %s", res.Status)
	}
}

func TestCompactOverflowShrinksTail(t *testing.T) {
	model := &stubModel{reply: "summary"}
	c, _ := New(Config{Model: model, OnContextOverflow: true, UncompactedRuns: 5})
	history := conversation(2, 10) // fewer runs than the configured tail
	res, err := c.Compact(context.Background(), history, Options{ForOverflow: true})
	if err != nil || !res.Compacted() {
		t.Fatalf("overflow fold must shrink the tail: %v %+v", err, res)
	}
}

func TestShouldCompactAndCounter(t *testing.T) {
	c, _ := New(Config{CompactAtTokens: 10})
	if c.ShouldCompact(conversation(1, 1)) {
		t.Fatal("small request must not trigger")
	}
	if !c.ShouldCompact(conversation(3, 20)) {
		t.Fatal("large request must trigger")
	}
	counted, _ := New(Config{CompactAtTokens: 10, TokenCounter: func([]models.Message) int { return 1000 }})
	if !counted.ShouldCompact(conversation(1, 1)) {
		t.Fatal("custom counter must be used")
	}
}

func TestFitToBudget(t *testing.T) {
	c, _ := New(Config{CompactedTokenBudget: 20})
	summary := "## Goal\n" + strings.Repeat("word ", 10) + "\n## Completed\n" + strings.Repeat("done ", 50)
	got := c.fitToBudget(summary)
	if EstimateTextTokens(got) > 20 {
		t.Fatalf("summary over budget: %d tokens", EstimateTextTokens(got))
	}
	off := false
	c2, _ := New(Config{CompactedTokenBudget: 20, EnforceTokenBudget: &off})
	if c2.fitToBudget(summary) != summary {
		t.Fatal("budget must not be enforced when disabled")
	}
}

func TestRecordRoundTrip(t *testing.T) {
	r := &Record{ID: "x", Summary: "s", FirstKeptMessageID: "msg_1", MessagesCompacted: 3, CreatedAt: 1}
	back := RecordFromValue(RecordToMap(r))
	if back == nil || *back != *r {
		t.Fatalf("round trip mismatch: %+v", back)
	}
	if RecordFromValue(map[string]interface{}{"foo": 1}) != nil {
		t.Fatal("a record without a summary is not usable")
	}
}

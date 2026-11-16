package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devalexandre/agno-golang/agno/compaction"
	"github.com/devalexandre/agno-golang/agno/models"
	inmemory "github.com/devalexandre/agno-golang/agno/storage/in_memory"
)

// fakeModel is a scriptable model for runtime tests. handler decides the reply
// for each call; calls records every request.
type fakeModel struct {
	mu      sync.Mutex
	calls   [][]models.Message
	handler func(ctx context.Context, msgs []models.Message) (*models.MessageResponse, error)
}

func (f *fakeModel) Invoke(ctx context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]models.Message(nil), msgs...))
	f.mu.Unlock()
	return f.handler(ctx, msgs)
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
func (f *fakeModel) GetID() string { return "fake" }

func (f *fakeModel) lastCall() []models.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}

func isSummarizerCall(msgs []models.Message) bool {
	return len(msgs) > 0 && strings.Contains(msgs[0].Content, "running summary of a long conversation")
}

func isFollowupCall(msgs []models.Message) bool {
	return len(msgs) > 0 && strings.Contains(msgs[0].Content, "generate follow-up suggestions")
}

func reply(s string) (*models.MessageResponse, error) {
	return &models.MessageResponse{Role: "assistant", Content: s}, nil
}

// defaultHandler answers summaries, follow-ups and normal turns.
func defaultHandler(answer string) func(context.Context, []models.Message) (*models.MessageResponse, error) {
	return func(_ context.Context, msgs []models.Message) (*models.MessageResponse, error) {
		switch {
		case isSummarizerCall(msgs):
			return reply("## Goal\nchat with the user")
		case isFollowupCall(msgs):
			return reply(`{"suggestions": ["Tell me more", "Give an example", "Summarize it", "Extra one"]}`)
		default:
			return reply(answer)
		}
	}
}

func newTestAgent(t *testing.T, cfg AgentConfig) *Agent {
	t.Helper()
	a, err := NewAgent(cfg)
	if err != nil {
		t.Fatalf("NewAgent: %v", err)
	}
	return a
}

func countSummaries(msgs []models.Message) int {
	n := 0
	for _, m := range msgs {
		if compaction.IsInjectedSummary(m) {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Run status / cancellation
// ---------------------------------------------------------------------------

func TestRunStatusCompleted(t *testing.T) {
	m := &fakeModel{handler: defaultHandler("hello")}
	a := newTestAgent(t, AgentConfig{Model: m})
	resp, err := a.Run("hi", WithRunID("run-1"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != models.RunStatusCompleted || resp.CancellationStage != "" {
		t.Fatalf("want COMPLETED, got %q / %q", resp.Status, resp.CancellationStage)
	}
	if resp.RunID != "run-1" || resp.SessionID == "" {
		t.Fatalf("run/session id not set: %+v", resp)
	}
}

func TestRunStatusError(t *testing.T) {
	m := &fakeModel{handler: func(context.Context, []models.Message) (*models.MessageResponse, error) {
		return nil, errors.New("provider exploded")
	}}
	a := newTestAgent(t, AgentConfig{Model: m})
	resp, err := a.Run("hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if resp.Status != models.RunStatusError || resp.RunID == "" {
		t.Fatalf("want ERROR with run id, got %+v", resp)
	}
}

func TestRunCancelledWhileExecuting(t *testing.T) {
	started := make(chan struct{})
	m := &fakeModel{handler: func(ctx context.Context, _ []models.Message) (*models.MessageResponse, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	a := newTestAgent(t, AgentConfig{Model: m})
	go func() {
		<-started
		if !a.CancelRun("run-x") {
			t.Error("CancelRun must find the active run")
		}
	}()
	done := make(chan struct{})
	var resp models.RunResponse
	var err error
	go func() {
		resp, err = a.Run("hi", WithRunID("run-x"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run was not cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if resp.Status != models.RunStatusCancelled || resp.CancellationStage != models.CancellationStageExecuting {
		t.Fatalf("want CANCELLED/EXECUTING, got %q/%q", resp.Status, resp.CancellationStage)
	}
	if a.CancelRun("run-x") {
		t.Fatal("finished run must no longer be tracked")
	}
}

func TestRunCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &fakeModel{handler: defaultHandler("never")}
	a := newTestAgent(t, AgentConfig{Model: m, Context: ctx})
	resp, err := a.Run("hi")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if resp.Status != models.RunStatusCancelled || resp.CancellationStage != models.CancellationStagePending {
		t.Fatalf("want CANCELLED/PENDING, got %q/%q", resp.Status, resp.CancellationStage)
	}
	if len(m.calls) != 0 {
		t.Fatal("model must not be called for a run cancelled before it started")
	}
}

func TestRunOptionsAreApplied(t *testing.T) {
	m := &fakeModel{handler: defaultHandler("ok")}
	a := newTestAgent(t, AgentConfig{Model: m})
	resp, err := a.Run("hi", WithSessionID("sess-42"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.SessionID != "sess-42" {
		t.Fatalf("WithSessionID not applied: %q", resp.SessionID)
	}
}

// ---------------------------------------------------------------------------
// Compaction
// ---------------------------------------------------------------------------

func TestAutoCompactionFoldsHistory(t *testing.T) {
	long := strings.Repeat("lorem ipsum ", 40) // ~120 tokens per answer
	m := &fakeModel{handler: defaultHandler(long)}
	a := newTestAgent(t, AgentConfig{
		Model:                m,
		AddHistoryToMessages: true,
		Compaction:           &compaction.Config{CompactAtTokens: 600, UncompactedRuns: 1},
	})

	var compactedAt = -1
	for i := 0; i < 8; i++ {
		resp, err := a.Run("question number " + string(rune('a'+i)))
		if err != nil {
			t.Fatal(err)
		}
		if resp.Compaction != nil && compactedAt < 0 {
			compactedAt = i
			if resp.Compaction.MessagesCompacted == 0 || resp.Compaction.RunID != resp.RunID {
				t.Fatalf("bad compaction record: %+v", resp.Compaction)
			}
		}
	}
	if compactedAt < 0 {
		t.Fatal("expected a compaction to happen")
	}
	if a.GetCompactionRecord() == nil || a.GetCompactionStats().Compactions == 0 {
		t.Fatal("record in force / stats not set")
	}
	// The latest model request carries exactly one summary and stays bounded.
	var last []models.Message
	for i := len(m.calls) - 1; i >= 0; i-- {
		if !isSummarizerCall(m.calls[i]) {
			last = m.calls[i]
			break
		}
	}
	if countSummaries(last) != 1 {
		t.Fatalf("want one injected summary in the request, got %d", countSummaries(last))
	}
	if compaction.EstimateTokens(last) > 1200 {
		t.Fatalf("request not bounded by compaction: %d tokens", compaction.EstimateTokens(last))
	}
	if last[len(last)-1].Role != models.TypeUserRole {
		t.Fatal("current input must remain the last message")
	}
}

func TestOverflowRecoveryRetriesOnce(t *testing.T) {
	m := &fakeModel{}
	m.handler = func(_ context.Context, msgs []models.Message) (*models.MessageResponse, error) {
		if isSummarizerCall(msgs) {
			return reply("## Goal\nfolded")
		}
		if len(msgs) > 6 && countSummaries(msgs) == 0 {
			return nil, errors.New("error 400: context_length_exceeded")
		}
		return reply(strings.Repeat("a long answer ", 30))
	}
	a := newTestAgent(t, AgentConfig{
		Model:                m,
		AddHistoryToMessages: true,
		Compaction:           compaction.OverflowOnly(),
	})
	var resp models.RunResponse
	var err error
	for i := 0; i < 5; i++ {
		resp, err = a.Run("q")
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if resp.Compaction != nil {
			break
		}
	}
	if resp.Compaction == nil {
		t.Fatal("expected overflow recovery to compact")
	}
	if resp.Status != models.RunStatusCompleted {
		t.Fatalf("retried run must complete, got %s", resp.Status)
	}
}

func TestOverflowWithoutCompactionFails(t *testing.T) {
	m := &fakeModel{handler: func(context.Context, []models.Message) (*models.MessageResponse, error) {
		return nil, errors.New("maximum context length exceeded")
	}}
	a := newTestAgent(t, AgentConfig{Model: m, AddHistoryToMessages: true})
	if _, err := a.Run("q"); err == nil {
		t.Fatal("expected error")
	}
}

func TestManualCompactAndPersistence(t *testing.T) {
	db := inmemory.New()
	m := &fakeModel{handler: defaultHandler("an answer")}

	notEnabled := newTestAgent(t, AgentConfig{Model: m})
	res, err := notEnabled.Compact(context.Background())
	if err != nil || res.Status != compaction.StatusNotEnabled {
		t.Fatalf("want not_enabled, got %v %+v", err, res)
	}

	cfg := &compaction.Config{CompactAtTokens: compaction.NoThreshold, UncompactedRuns: 1}
	a := newTestAgent(t, AgentConfig{Model: m, DB: db, SessionID: "s1", AddHistoryToMessages: true, Compaction: cfg})
	res, _ = a.Compact(context.Background())
	if res.Status != compaction.StatusNoHistory {
		t.Fatalf("want no_history, got %s", res.Status)
	}
	for _, q := range []string{"first", "second", "third"} {
		if _, err := a.Run(q); err != nil {
			t.Fatal(err)
		}
	}
	res, err = a.Compact(context.Background())
	if err != nil || !res.Compacted() {
		t.Fatalf("manual compaction failed: %v %+v", err, res)
	}
	if res.Record.MessagesCompacted != 4 {
		t.Fatalf("want 4 folded messages, got %d", res.Record.MessagesCompacted)
	}

	// A new agent on the same session restores the summary and the cut.
	b := newTestAgent(t, AgentConfig{Model: m, DB: db, SessionID: "s1", AddHistoryToMessages: true, Compaction: cfg})
	if b.GetCompactionRecord() == nil || b.GetCompactionRecord().Summary != res.Record.Summary {
		t.Fatal("compaction record not restored from session")
	}
	if _, err := b.Run("fourth"); err != nil {
		t.Fatal(err)
	}
	req := m.lastCall()
	if countSummaries(req) != 1 {
		t.Fatalf("restored request must carry the summary, got %d", countSummaries(req))
	}
	for _, msg := range req {
		if msg.Content == "first" || msg.Content == "second" {
			t.Fatalf("folded message %q must not be replayed", msg.Content)
		}
	}
	found := false
	for _, msg := range req {
		if msg.Content == "third" {
			found = true
		}
	}
	if !found {
		t.Fatal("kept run must be replayed")
	}
}

// ---------------------------------------------------------------------------
// Follow-ups
// ---------------------------------------------------------------------------

func TestFollowupsGenerated(t *testing.T) {
	m := &fakeModel{handler: defaultHandler("Paris is the capital of France.")}
	a := newTestAgent(t, AgentConfig{Model: m, Followups: &FollowupConfig{}})
	resp, err := a.Run("capital of France?")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Followups) != 3 || resp.Followups[0] != "Tell me more" {
		t.Fatalf("want 3 followups (max enforced), got %v", resp.Followups)
	}
	req := m.lastCall()
	if !strings.Contains(req[1].Content, "User message:\ncapital of France?") ||
		!strings.Contains(req[1].Content, "Generate exactly 3 follow-up suggestions.") {
		t.Fatalf("unexpected followup prompt: %s", req[1].Content)
	}
}

func TestFollowupsFailureDoesNotFailRun(t *testing.T) {
	m := &fakeModel{handler: func(_ context.Context, msgs []models.Message) (*models.MessageResponse, error) {
		if isFollowupCall(msgs) {
			return reply("not json at all")
		}
		return reply("answer")
	}}
	a := newTestAgent(t, AgentConfig{Model: m, Followups: &FollowupConfig{MaxFollowups: 2}})
	resp, err := a.Run("q")
	if err != nil || resp.Status != models.RunStatusCompleted || resp.Followups != nil {
		t.Fatalf("followup failure must be silent: %v %+v", err, resp)
	}
}

func TestFollowupConfigCountRange(t *testing.T) {
	cases := []struct {
		cfg      FollowupConfig
		min, max int
		wantErr  bool
	}{
		{FollowupConfig{}, 3, 3, false},
		{FollowupConfig{NumFollowups: 5}, 5, 5, false},
		{FollowupConfig{MaxFollowups: 4}, 0, 4, false},
		{FollowupConfig{MinFollowups: 2}, 2, 3, false},
		{FollowupConfig{NumFollowups: 2, MaxFollowups: 3}, 0, 0, true},
		{FollowupConfig{MinFollowups: 5, MaxFollowups: 3}, 0, 0, true},
	}
	for _, c := range cases {
		mn, mx, err := c.cfg.CountRange()
		if (err != nil) != c.wantErr || (!c.wantErr && (mn != c.min || mx != c.max)) {
			t.Errorf("%+v: got %d,%d,%v", c.cfg, mn, mx, err)
		}
	}
	if _, err := NewAgent(AgentConfig{Model: &fakeModel{}, Followups: &FollowupConfig{NumFollowups: -1}}); err == nil {
		t.Fatal("invalid followup config must fail NewAgent")
	}
}

func TestParseFollowups(t *testing.T) {
	got, err := ParseFollowups("```json\n{\"suggestions\": [\"a\", \" \", \"b\"]}\n```", 0)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v %v", got, err)
	}
	got, err = ParseFollowups(`["x","y","z"]`, 2)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %v %v", got, err)
	}
	got, err = ParseFollowups(`Sure! {"suggestions": ["only"]}`, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v %v", got, err)
	}
}

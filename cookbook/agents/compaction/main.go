// Conversation compaction: older runs of a long session are folded into a
// generated summary so the request sent to the model stays bounded.
//
// Shows two modes on AgentConfig.Compaction (agno/compaction.Config):
//
//  1. automatic: CompactAtTokens triggers a fold before a run whose request
//     crosses the threshold; the run's RunResponse.Compaction describes it;
//  2. manual: CompactAtTokens = compaction.NoThreshold and Agent.Compact(ctx)
//     folds on demand. GetCompactionRecord / GetCompactionStats report it.
//
// Runs offline: a small in-file stub model answers both the normal turns and
// the summarizer call. Swap it for any real model (e.g. ollama.NewOllamaChat).
//
//	go run ./cookbook/agents/compaction
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/compaction"
	"github.com/devalexandre/agno-golang/agno/models"
)

func main() {
	ctx := context.Background()
	model := &stubModel{}

	// --- 1. Automatic compaction -----------------------------------------
	fmt.Println("== automatic compaction (CompactAtTokens: 1500, UncompactedRuns: 1)")
	auto, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Model:                model,
		Name:                 "auto-compacting",
		AddHistoryToMessages: true,
		Compaction: &compaction.Config{
			CompactAtTokens: 1500, // small threshold so the demo folds quickly
			UncompactedRuns: 1,    // keep only the latest run verbatim
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	for i := 1; i <= 6; i++ {
		resp, err := auto.Run(fmt.Sprintf("Question %d: tell me about topic %d", i, i))
		if err != nil {
			log.Fatal(err)
		}
		if resp.Compaction != nil {
			fmt.Printf("run %d: folded %d messages (%d -> %d tokens)\n",
				i, resp.Compaction.MessagesCompacted, resp.Compaction.TokensBefore, resp.Compaction.TokensAfter)
		} else {
			fmt.Printf("run %d: no compaction\n", i)
		}
	}
	printStats(auto)

	// --- 2. Manual compaction --------------------------------------------
	fmt.Println("\n== manual compaction (NoThreshold + Agent.Compact)")
	manual, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Model:                model,
		Name:                 "manual-compacting",
		SessionID:            "compaction-demo",
		AddHistoryToMessages: true,
		Compaction: &compaction.Config{
			CompactAtTokens: compaction.NoThreshold,
			UncompactedRuns: 1,
			Instructions:    "Keep any numbers the user mentioned.",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	res, _ := manual.Compact(ctx) // nothing to fold yet
	fmt.Printf("before any run: %s (%s)\n", res.Status, res.Message)

	for _, q := range []string{"My budget is 300 euros.", "I prefer trains.", "Plan a weekend in Porto."} {
		if _, err := manual.Run(q); err != nil {
			log.Fatal(err)
		}
	}
	res, err = manual.Compact(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("after 3 runs: %s\n", res.Status)
	if res.Compacted() {
		r := manual.GetCompactionRecord()
		fmt.Printf("record: id=%s messages_compacted=%d tokens %d -> %d\nsummary:\n%s\n",
			r.ID, r.MessagesCompacted, r.TokensBefore, r.TokensAfter, r.Summary)
	}
	printStats(manual)
}

func printStats(a *agent.Agent) {
	s := a.GetCompactionStats()
	fmt.Printf("stats: compactions=%d messages_compacted=%d tokens_before=%d tokens_after=%d\n",
		s.Compactions, s.MessagesCompacted, s.TokensBefore, s.TokensAfter)
}

// stubModel is an offline model: it writes a short summary when called by the
// compactor and a long-ish answer otherwise, so the history grows quickly.
type stubModel struct{}

func (stubModel) GetID() string { return "stub-model" }

func (stubModel) Invoke(_ context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	if len(msgs) > 0 && strings.Contains(msgs[0].Content, "running summary of a long conversation") {
		return &models.MessageResponse{Role: "assistant", Content: "## Goal\nThe user is planning a trip.\n## Facts\n- budget: 300 euros\n- prefers trains"}, nil
	}
	return &models.MessageResponse{Role: "assistant", Content: strings.Repeat("Here is a detailed answer. ", 20)}, nil
}

func (m stubModel) AInvoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch, errCh := make(chan *models.MessageResponse, 1), make(chan error, 1)
	r, err := m.Invoke(ctx, msgs, opts...)
	if err != nil {
		errCh <- err
	} else {
		ch <- r
	}
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m stubModel) InvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range opts {
		o(call)
	}
	r, err := m.Invoke(ctx, msgs)
	if err != nil {
		return err
	}
	if call.StreamingFunc != nil {
		return call.StreamingFunc(ctx, []byte(r.Content))
	}
	return nil
}

func (m stubModel) AInvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return m.AInvoke(ctx, msgs, opts...)
}

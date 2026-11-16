// Typed run metrics (agno/metrics).
//
// Every agent run now fills RunResponse.RunMetrics (typed, with a per-model-
// type breakdown: model, output_model, memory_model, compression_model,
// compaction_model, ...) and the same numbers in the RunResponse.Metrics map.
// Sessions aggregate runs with metrics.SessionMetrics.
//
// Offline by default with a stub model that reports usage. Set
// OPENAI_API_KEY to run the same flow against OpenAI (gpt-4o-mini), whose
// client reports real token usage, accumulated across its tool loop.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/metrics"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
)

// stubModel reports token usage like a real provider would.
type stubModel struct{}

func (stubModel) Invoke(_ context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	in := 0
	for _, m := range msgs {
		in += len(m.Content) / 4
	}
	cost := 0.0001
	return &models.MessageResponse{Role: "assistant", Content: "Brasília is the capital of Brazil.",
		Metrics: &metrics.MessageMetrics{Tokens: metrics.Tokens{InputTokens: in, OutputTokens: 9, TotalTokens: in + 9, Cost: &cost}}}, nil
}
func (stubModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }
func (stubModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) GetID() string { return "stub-model" }

func main() {
	var model models.AgnoModelInterface = stubModel{}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		m, err := chat.NewOpenAIChat(models.WithID("gpt-4o-mini"), models.WithAPIKey(key))
		if err != nil {
			log.Fatal(err)
		}
		model = m
	}

	a, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Model: model})
	if err != nil {
		log.Fatal(err)
	}

	var session metrics.SessionMetrics
	for _, q := range []string{"What is the capital of Brazil?", "And its population?"} {
		resp, err := a.Run(q)
		if err != nil {
			log.Fatal(err)
		}
		rm := resp.RunMetrics
		fmt.Printf("run %s: input=%d output=%d total=%d duration=%s\n", resp.RunID[:8], rm.InputTokens, rm.OutputTokens, rm.TotalTokens, rm.Duration)
		for modelType, entries := range rm.Snapshot().Details {
			for _, e := range entries {
				fmt.Printf("  %-12s %s/%s total=%d\n", modelType, e.Provider, e.ID, e.TotalTokens)
			}
		}
		session.AccumulateFromRun(rm)
	}

	data, _ := json.MarshalIndent(session, "", "  ")
	fmt.Printf("session totals:\n%s\n", data)
}

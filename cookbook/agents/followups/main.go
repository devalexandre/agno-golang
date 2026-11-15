// Follow-up suggestions: after each run the agent asks a model for short
// prompts the user could send next and returns them in RunResponse.Followups.
//
// AgentConfig.Followups (agent.FollowupConfig) controls the count
// (NumFollowups, or MinFollowups/MaxFollowups; default 3), optional extra
// Instructions and an optional separate Model (nil = the agent's model).
// A failed or malformed follow-up call never fails the run: Followups is
// simply empty.
//
// Runs offline: the in-file stub model answers the question and returns a JSON
// list for the follow-up call. Swap it for any real model.
//
//	go run ./cookbook/agents/followups
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
)

func main() {
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      context.Background(),
		Model:        stubModel{},
		Name:         "Travel Assistant",
		Instructions: "You are a concise travel assistant.",
		Followups: &agent.FollowupConfig{
			MaxFollowups: 4,
			Instructions: "Suggestions must be about travelling in Portugal.",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := ag.Run("What is the best time to visit Lisbon?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Answer:", resp.TextContent)
	fmt.Println("Follow-up suggestions:")
	for i, f := range resp.Followups {
		fmt.Printf("  %d. %s\n", i+1, f)
	}

	// The parser used for the follow-up reply is exported, e.g. for custom flows.
	parsed, _ := agent.ParseFollowups("```json\n[\"a\", \"b\", \"c\"]\n```", 2)
	fmt.Println("ParseFollowups with max 2:", parsed)
}

// stubModel answers normal turns and returns JSON for the follow-up request,
// recognised by its system prompt.
type stubModel struct{}

func (stubModel) GetID() string { return "stub-model" }

func (stubModel) Invoke(_ context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	if len(msgs) > 0 && strings.Contains(msgs[0].Content, "generate follow-up suggestions") {
		return &models.MessageResponse{Role: "assistant", Content: `{"suggestions": [
			"What are the must-see neighbourhoods in Lisbon?",
			"How do I get from Lisbon to Sintra?",
			"Which festivals happen in June?",
			"Is Porto worth a day trip?",
			"A fifth one the MaxFollowups cap drops"]}`}, nil
	}
	return &models.MessageResponse{Role: "assistant", Content: "Spring (March-May) and autumn (September-October): mild weather and fewer crowds."}, nil
}

func (m stubModel) AInvoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch, errCh := make(chan *models.MessageResponse, 1), make(chan error, 1)
	r, _ := m.Invoke(ctx, msgs, opts...)
	ch <- r
	close(ch)
	close(errCh)
	return ch, errCh
}

func (m stubModel) InvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) error {
	call := &models.CallOptions{}
	for _, o := range opts {
		o(call)
	}
	r, _ := m.Invoke(ctx, msgs)
	if call.StreamingFunc != nil {
		return call.StreamingFunc(ctx, []byte(r.Content))
	}
	return nil
}

func (m stubModel) AInvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return m.AInvoke(ctx, msgs, opts...)
}

// Tool-result compression (agno/compression).
//
// A compression.Manager uses a (cheaper) model to shrink tool results while
// keeping facts, numbers, dates and ids. With AgentConfig.CompressionManager
// every oversized tool result is compressed before the main model sees it,
// and the compression model's usage lands in the run's typed metrics under
// "compression_model". The Manager can also compress the tool messages of a
// transcript once a count threshold is hit (ShouldCompress / Compress).
//
// Offline by default with a stub compression model. Set OPENAI_API_KEY to
// use gpt-4o-mini as the compression model instead.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/compression"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// stubModel "compresses" by keeping lines containing digits.
type stubModel struct{}

func (stubModel) Invoke(_ context.Context, msgs []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	last := msgs[len(msgs)-1].Content
	var kept []string
	for _, l := range strings.Split(last, "\n") {
		if strings.ContainsAny(l, "0123456789") {
			kept = append(kept, strings.TrimSpace(l))
		}
	}
	return &models.MessageResponse{Role: "assistant", Content: strings.Join(kept, "; ")}, nil
}
func (stubModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }
func (stubModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (stubModel) GetID() string { return "stub-compressor" }

type newsTool struct{ toolkit.Toolkit }

type newsParams struct {
	Topic string `json:"topic" description:"Topic" required:"true"`
}

func (n *newsTool) Search(p newsParams) (interface{}, error) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString("According to recent market analysis and industry reports, there is a lot of commentary about " + p.Topic + ".\n")
	}
	b.WriteString("OpenAI announced ChatGPT Atlas on Oct 21 2025.\nApps in ChatGPT launched Oct 6 2025 with partners Spotify, Zillow, Canva.\n")
	return b.String(), nil
}

func main() {
	var compressor models.AgnoModelInterface = stubModel{}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		m, err := chat.NewOpenAIChat(models.WithID("gpt-4o-mini"), models.WithAPIKey(key))
		if err != nil {
			log.Fatal(err)
		}
		compressor = m
	}
	manager := compression.NewManager(compressor)
	manager.MinChars = 1000

	nt := &newsTool{Toolkit: toolkit.NewToolkit()}
	nt.Name = "news"
	nt.Register("search", "Search news", nt, nt.Search, newsParams{})

	a, err := agent.NewAgent(agent.AgentConfig{
		Context:            context.Background(),
		Model:              stubModel{},
		Tools:              []toolkit.Tool{nt},
		CompressionManager: manager,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range a.GetTools() {
		if _, ok := t.GetMethods()["news_search"]; ok {
			out, _ := t.Execute("news_search", json.RawMessage(`{"topic":"OpenAI"}`))
			fmt.Printf("compressed tool result: %s\n", out)
		}
	}
	fmt.Printf("stats: %v\n", manager.Stats())

	// Transcript mode: compress once 3 uncompressed tool results accumulate.
	id := func(s string) *string { return &s }
	transcript := []models.Message{
		{Role: models.TypeUserRole, Content: "Compare the three reports"},
		{Role: models.TypeToolRole, ToolCallID: id("1"), Content: "Report A says revenue grew 12% in 2025 according to analysts."},
		{Role: models.TypeToolRole, ToolCallID: id("2"), Content: "Report B notes 3 new products launched in Q3."},
		{Role: models.TypeToolRole, ToolCallID: id("3"), Content: "Report C counts 1500 employees as of Jan 2026."},
	}
	if manager.ShouldCompress(transcript) {
		manager.Compress(context.Background(), transcript, nil)
	}
	for _, m := range transcript[1:] {
		fmt.Printf("tool message: %s\n", m.Content)
	}
}

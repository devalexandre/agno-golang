// Example: model fallback with agno-golang.
//
// A FallbackModel wraps a primary model and transparently retries with
// configured fallback models when the primary fails — with separate lists for
// rate-limit and context-overflow errors. It implements the model interface,
// so it can be passed to an agent like any other model.
//
//	export ANTHROPIC_API_KEY="sk-ant-..."
//	export OPENAI_API_KEY="sk-..."
//	go run ./cookbook/models/fallback
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/anthropic"
	"github.com/devalexandre/agno-golang/agno/models/groq"
	openai "github.com/devalexandre/agno-golang/agno/models/openai/chat"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()
	utils.SetMarkdownMode(true)

	primary, err := anthropic.NewAnthropic(models.WithID("claude-sonnet-4-5"))
	if err != nil {
		log.Fatal(err)
	}
	gpt, err := openai.NewOpenAIChat(models.WithID("gpt-4o-mini"))
	if err != nil {
		log.Fatal(err)
	}
	fast, err := groq.NewGroqChat(models.WithID(groq.ModelLlama31_8BInstant))
	if err != nil {
		log.Fatal(err)
	}

	// Build a fallback model:
	//   - generic failures  -> try GPT-4o-mini
	//   - rate-limit (429)   -> try the fast Groq model
	fb := models.NewFallbackModel(primary, models.FallbackConfig{
		OnError:     []models.AgnoModelInterface{gpt},
		OnRateLimit: []models.AgnoModelInterface{fast},
		Callback: func(primaryID, fallbackID string, err error) {
			fmt.Printf("⚠️  %s failed (%v) — falling back to %s\n", primaryID, err, fallbackID)
		},
	})

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        fb,
		Name:         "ResilientAssistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Give me one tip for writing resilient distributed systems.", true, true)

	m := fb.Metrics()
	fmt.Printf("\nMetrics: requests=%d successes=%d failures=%d fallbacks=%d\n",
		m.Requests, m.Successes, m.Failures, m.FallbackActivations)
}

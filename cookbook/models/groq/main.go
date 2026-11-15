// Example: using Groq (OpenAI-compatible) with agno-golang.
//
// Requires the GROQ_API_KEY environment variable:
//
//	export GROQ_API_KEY="gsk_..."
//	go run ./cookbook/models/groq
//
// The same pattern works for the other OpenAI-compatible providers added in
// Phase 1: mistral, xai, together, fireworks and perplexity — just swap the
// import and constructor (e.g. mistral.NewMistralChat, xai.NewXaiChat).
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/groq"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()
	utils.SetMarkdownMode(true)

	// API key is read from GROQ_API_KEY when WithAPIKey is not provided.
	model, err := groq.NewGroqChat(
		models.WithID(groq.ModelLlama33_70BVersatile),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "GroqAssistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain what makes Groq inference fast in two sentences.", true, true)
}

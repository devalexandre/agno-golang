// Example: using the OpenAI Responses API with agno-golang.
//
// API key: set OPENAI_API_KEY (export OPENAI_API_KEY=...).
// Default model: gpt-5.4-mini
//
//	go run ./cookbook/models/openai_responses
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/responses"
)

func main() {
	ctx := context.Background()

	model, err := responses.NewOpenAIResponses(
		models.WithID(responses.DefaultModel),
		models.WithClientParams(map[string]interface{}{
			"reasoning_effort":  "low",
			"reasoning_summary": "auto",
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "the OpenAI Responses API Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

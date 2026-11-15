// Example: using Tuning Engines with agno-golang.
//
// API key: set TUNING_ENGINES_API_KEY (optional TUNING_ENGINES_BASE_URL).
// Default model: gpt-4o
//
//	go run ./cookbook/models/tuning_engines
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	tuningengines "github.com/devalexandre/agno-golang/agno/models/tuning_engines"
)

func main() {
	ctx := context.Background()

	model, err := tuningengines.NewTuningEnginesChat(
		models.WithID(tuningengines.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Tuning Engines Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

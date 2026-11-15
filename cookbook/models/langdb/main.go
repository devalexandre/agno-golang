// Example: using LangDB with agno-golang.
//
// API key: set LANGDB_API_KEY and LANGDB_PROJECT_ID (optional LANGDB_API_BASE_URL).
// Default model: gpt-4o
//
//	go run ./cookbook/models/langdb
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/langdb"
)

func main() {
	ctx := context.Background()

	model, err := langdb.NewLangDBChat(
		models.WithID(langdb.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "LangDB Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

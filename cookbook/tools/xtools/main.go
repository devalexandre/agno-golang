// Example: searching and posting on X with the X toolkit.
//
// Requires X_BEARER_TOKEN for reads and X_CONSUMER_KEY, X_CONSUMER_SECRET,
// X_ACCESS_TOKEN, X_ACCESS_TOKEN_SECRET for posting / DMs / timeline.
//
//	go run ./cookbook/tools/xtools
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/xtools"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	x := xtools.New(xtools.WithIncludePostMetrics(true))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "X Assistant",
		Model:         model,
		Instructions:  "You research conversations on X and summarize them with links to the posts.",
		Tools:         []toolkit.Tool{x},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Search recent posts about #golang and summarize the most liked ones.", true, true)
}

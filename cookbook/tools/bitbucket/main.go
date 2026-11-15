// Example: inspect a Bitbucket Cloud repository.
//
// Requires BITBUCKET_USERNAME and BITBUCKET_TOKEN (or BITBUCKET_PASSWORD).
//
//	go run ./cookbook/tools/bitbucket
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/bitbucket"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
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

	bb := bitbucket.New(
		bitbucket.WithWorkspace("my-workspace"),
		bitbucket.WithRepoSlug("my-repo"),
	)

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Bitbucket Assistant",
		Model:         model,
		Instructions:  "Use the Bitbucket tools to answer questions about the repository.",
		Tools:         []toolkit.Tool{bb},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Summarize the open pull requests and the last 5 commits.", true, true)
}

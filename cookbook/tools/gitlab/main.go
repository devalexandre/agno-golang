// Example: browse GitLab projects, merge requests and issues.
//
// Requires GITLAB_ACCESS_TOKEN (optional for public projects) and optionally
// GITLAB_BASE_URL for self-hosted instances.
//
//	go run ./cookbook/tools/gitlab
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/gitlab"
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

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "GitLab Assistant",
		Model:         model,
		Instructions:  "Use the GitLab tools to answer questions about projects, merge requests and issues.",
		Tools:         []toolkit.Tool{gitlab.New()},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List the 5 most recent open merge requests in gitlab-org/gitlab.", true, true)
}

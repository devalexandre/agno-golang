// Example: let an agent run a workflow with WorkflowTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
// The demo wraps a plain Go function. To expose a workflow/v2 workflow use
// workflowtools.NewWorkflowTools(wf) instead.
//
//	go run ./cookbook/tools/workflowtools
package main

import (
	"context"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/workflowtools"
)

// slugWorkflow is a tiny two-step "workflow": normalise then slugify a title.
func slugWorkflow(_ context.Context, input string, _ map[string]interface{}) (interface{}, error) {
	normalised := strings.Join(strings.Fields(strings.ToLower(input)), " ")
	return map[string]interface{}{
		"steps":   []string{"normalise", "slugify"},
		"content": strings.ReplaceAll(normalised, " ", "-"),
	}, nil
}

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	wt, err := workflowtools.NewWorkflowToolsFromFunc(slugWorkflow, workflowtools.WithAll())
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Workflow Runner",
		Model:         model,
		Instructions:  wt.Instructions,
		Tools:         []toolkit.Tool{wt},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("Create a URL slug for the blog post title 'Ten Tips For   Writing Go Toolkits'.", true, true)
}

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	// 1. Initialize the model (local Ollama)
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Initialize the Git tool (REAL Git operations)
	gitTool := tools.NewGitTool()

	// 3. Create the Git Version Control Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Git Version Control Expert",
		Model:         model,
		Instructions:  "You are a Git version control expert. Use the Git tool methods to manage repositories: InitRepository for creating repos, GetStatus to check status, GetLog to view history, CreateCommit to make commits, CreateBranch for branching, PullChanges and PushChanges for remote operations. Always specify the repository path.",
		Tools:         []toolkit.Tool{gitTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Git Version Control with Real Operations ===")
	fmt.Println()

	// Example queries - specific and actionable
	queries := []string{
		"Initialize a new git repository at /tmp/my-project",
		"Get the git log with last 3 commits from /tmp/my-project",
		"Check the git status of /tmp/my-project repository",
	}

	for _, query := range queries {
		fmt.Printf("🔀 Query: %s\n", query)
		response, err := ag.Run(query)
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}
		fmt.Println("📋 Response:")
		fmt.Println(response.TextContent)
		fmt.Println("\n" + string([]byte{45, 45, 45, 45, 45, 45, 45, 45, 45, 45}) + "\n")
	}
}

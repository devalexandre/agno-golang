// Example: using Anthropic's Claude models with agno-golang.
//
// Requires the ANTHROPIC_API_KEY environment variable:
//
//	export ANTHROPIC_API_KEY="sk-ant-..."
//	go run ./cookbook/models/anthropic
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/anthropic"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()
	utils.SetMarkdownMode(true)

	// Create the Anthropic (Claude) model. The API key is read from
	// ANTHROPIC_API_KEY when WithAPIKey is not provided.
	model, err := anthropic.NewAnthropic(
		models.WithID("claude-sonnet-4-5"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// A simple tool to demonstrate tool-calling with Claude.
	searchTool := tools.NewDuckDuckGoTool()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "ClaudeAssistant",
		Instructions: `You are a concise, helpful assistant powered by Claude.
Use the search tool when you need up-to-date information, then summarize clearly.`,
		Tools:    []toolkit.Tool{searchTool},
		Markdown: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Simple completion ===")
	ag.PrintResponse("Explain what an AI agent is in two sentences.", true, true)

	fmt.Println("\n=== Tool calling ===")
	ag.PrintResponse("Search for the latest news about the Go programming language and summarize it.", true, true)
}

// Example: using SiliconFlow with agno-golang.
//
// API key: set SILICONFLOW_API_KEY (export SILICONFLOW_API_KEY=...).
// Default model: Qwen/QwQ-32B
//
//	go run ./cookbook/models/siliconflow
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/siliconflow"
)

func main() {
	ctx := context.Background()

	model, err := siliconflow.NewSiliconflowChat(
		models.WithID(siliconflow.DefaultModel),
	)
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "SiliconFlow Assistant",
		Instructions: "You are a concise, helpful assistant.",
		Markdown:     true,
	})
	if err != nil {
		log.Fatal(err)
	}

	ag.PrintResponse("Explain the Go proverb 'Clear is better than clever' in two sentences.", true, true)
}

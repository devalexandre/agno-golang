// Example: session memory and user graph search with ZepTools.
//
// Requires ZEP_API_KEY (Zep Cloud).
//
//	go run ./cookbook/tools/zep
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/zep"
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

	tk := zep.NewZepTools(zep.WithZepUserID("zep-demo-user"), zep.WithZepSessionID("zep-demo-session"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Zep Memory Agent",
		Model:         model,
		Instructions:  tk.Instructions(),
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("My name is John and I work as a data engineer. Save this and tell me what you know about me.", true, true)
}

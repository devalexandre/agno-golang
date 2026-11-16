// Example: SMS, calls and number lookups with the Plivo toolkit.
//
// Requires PLIVO_AUTH_ID and PLIVO_AUTH_TOKEN.
//
//	go run ./cookbook/tools/plivo
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/plivo"
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

	pl := plivo.New(plivo.WithEnableMakeCall(false))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Plivo Assistant",
		Model:         model,
		Instructions:  "You help with SMS and call activity on Plivo.",
		Tools:         []toolkit.Tool{pl},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List my 5 most recent outbound messages and look up the carrier of +14155552671.", true, true)
}

// Example: Scheduling Assistant with the calcom toolkit.
//
// Requires CALCOM_API_KEY and CALCOM_EVENT_TYPE_ID.
//
//	go run ./cookbook/tools/calcom
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/calcom"
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

	tools := calcom.New(calcom.WithUserTimezone("America/New_York"))

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Scheduling Assistant",
		Model:         model,
		Instructions:  "You book meetings on Cal.com. Check availability before booking.",
		Tools:         []toolkit.Tool{tools},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("What slots are available next Monday? Book the first one for Jane Doe (jane@example.com).", true, true)
}

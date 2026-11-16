// Example: accurate routing distances with OpenRouteServiceTools.
//
// Requires ORS_API_KEY (free token at https://account.heigit.org).
//
//	go run ./cookbook/tools/openrouteservice
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/openrouteservice"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Route Planner",
		Model:         model,
		Instructions:  "Never estimate distances; always use the routing tools.",
		Tools:         []toolkit.Tool{openrouteservice.NewOpenRouteServiceTools()},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("How far is it to drive from Berlin to Amsterdam, and how long by bike?", true, true)
}

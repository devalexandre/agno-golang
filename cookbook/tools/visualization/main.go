// Example: save SVG charts with VisualizationTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
//	go run ./cookbook/tools/visualization
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/tools/visualization"
)

func main() {
	viz, err := visualization.NewVisualizationTools(visualization.WithOutputDir("charts"))
	if err != nil {
		log.Fatal(err)
	}
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Visualizer",
		Model:         model,
		Tools:         []toolkit.Tool{viz},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Create a bar chart of sales by region: North 120, South 90, East 150, West 70.", true, true)
}

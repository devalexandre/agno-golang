// Example: generate report files with FileGenerationTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434.
//
//	go run ./cookbook/tools/filegeneration
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/filegeneration"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	files, err := filegeneration.NewFileGenerationTools(filegeneration.WithOutputDirectory("generated"))
	if err != nil {
		log.Fatal(err)
	}
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatal(err)
	}
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Name:          "Report Writer",
		Model:         model,
		Tools:         []toolkit.Tool{files},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	ag.PrintResponse("Write a short PDF report about the benefits of Go, and a CSV listing 3 Go web frameworks with their GitHub URLs.", true, true)
}

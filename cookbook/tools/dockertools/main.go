// Example: manage Docker through the Engine API with DockerTools.
//
// Requires a running Docker daemon (socket auto-detected, or DOCKER_HOST).
// Remove operations require user confirmation.
//
//	go run ./cookbook/tools/dockertools
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/dockertools"
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

	tk := dockertools.NewDockerTools()

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Docker Agent",
		Model:         model,
		Instructions:  "Manage Docker resources. Always list before changing anything.",
		Tools:         []toolkit.Tool{tk},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("List my running containers and the images available locally.", true, true)
}

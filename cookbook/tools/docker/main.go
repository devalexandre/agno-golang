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

	// 2. Initialize the DockerContainerManager tool (REAL Docker command execution)
	dockerTool := tools.NewDockerContainerManager()

	// 3. Create the Docker Management Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Docker Container Manager",
		Model:         model,
		Instructions:  "You are a Docker container management expert. Use the Docker management tools available:\n- pull_image: Pull an image (use image_name parameter like 'nginx:latest')\n- list_containers: List all containers\n- list_images: List all images\n- run_container: Start a new container\n- stop_container: Stop a container\n- remove_container: Remove a container\n- get_container_logs: Get container logs\n- get_container_stats: Get container stats\n\nAll tools execute REAL Docker commands on the system.",
		Tools:         []toolkit.Tool{dockerTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Docker Container Management Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Pull the Alpine Linux image (alpine:latest) from Docker Hub",
		"List all Docker containers including stopped ones",
		"List all Docker images with their sizes and creation dates",
	}

	for _, query := range queries {
		fmt.Printf("🐋 Query: %s\n", query)
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

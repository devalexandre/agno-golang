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

	// 2. Initialize the Kubernetes tool (REAL kubectl operations)
	kubeTool := tools.NewKubernetesOperationsTool()

	// 3. Create the Kubernetes Management Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Kubernetes Operations Manager",
		Model:         model,
		Instructions:  "You are a Kubernetes operations expert. Use the KubernetesOperationsTool methods to manage clusters: Version to check kubectl version, GetNamespaces to list namespaces, GetNodes for cluster nodes, GetPods to list pods, GetServices for services, DescribeResource for detailed info, and GetLogs for pod logs. Always specify required parameters like namespace and resource names.",
		Tools:         []toolkit.Tool{kubeTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Kubernetes Operations Example ===")
	fmt.Println()

	// Example queries
	queries := []string{
		"Check kubectl version and show cluster connection info",
		"List all namespaces using kubectl get namespaces",
		"Get information about all nodes in the cluster",
	}

	for _, query := range queries {
		fmt.Printf("☸️  Query: %s\n", query)
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

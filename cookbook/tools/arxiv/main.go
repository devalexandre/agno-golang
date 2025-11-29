package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	// 1. Initialize the model
	apiKey := os.Getenv("OLLAMA_API_KEY")
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Initialize the Arxiv tool
	arxivTool := tools.NewArxivTool(3) // Fetch top 3 results

	// 3. Create the Researcher Agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Academic Researcher",
		Model:         model,
		Instructions:  "You are an academic researcher. Search for papers on Arxiv to answer questions. Summarize the findings and cite the papers.",
		Tools:         []toolkit.Tool{arxivTool},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// 4. Run the agent
	fmt.Println("=== Arxiv Research Example ===")

	topic := "Large Language Models Reasoning"
	fmt.Printf("\n🔎 Researching: %s\n", topic)

	response, err := ag.Run(fmt.Sprintf("Find recent papers about '%s' and summarize the key approaches.", topic))
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("\n📝 Research Summary:")
	fmt.Println(response.TextContent)
}

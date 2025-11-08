package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(os.Getenv("OLLAMA_API_KEY")),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	assistant, err := agent.NewAgent(agent.AgentConfig{
		Model:        ollamaModel,
		Context:      context.Background(),
		Name:         "Assistente",
		Description:  "Trovador",
		Instructions: "Create a musical poem  about the given topic.",
		Markdown:     true,
		Debug:        false, // Enable debug to see the request
		// EnableSemanticCompression: true, // Enable semantic compression
		// SemanticModel:             ollamaModel,
		// SemanticMaxTokens:         200,
		Stream: true,
	})

	prompt := "Write a short poem in  about the sea and its mysteries."
	if err != nil {
		log.Fatalf("Failed to create assistant agent: %v", err)
	}

	response, err := assistant.Run(prompt)
	if err != nil {
		log.Fatalf("Failed to run assistant agent: %v", err)
	}
	log.Printf("Compressed Output:\n%s", response)
}

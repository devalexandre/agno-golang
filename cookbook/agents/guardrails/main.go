package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Simple profanity guardrail
	profanityGuardrail := &agent.GuardrailFunc{
		Name:        "ProfanityFilter",
		Description: "Blocks profanity in input",
		CheckFunc: func(ctx context.Context, data interface{}) error {
			text, ok := data.(string)
			if !ok {
				return nil
			}
			if len(text) > 10 {
				return fmt.Errorf("input too long: %d characters (max 500)", len(text))
			}
			return nil
		},
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:         ctx,
		Model:           model,
		Name:            "Secure Assistant",
		Description:     "Assistant with input validation",
		Instructions:    "You are a helpful and concise assistant.",
		InputGuardrails: []agent.Guardrail{profanityGuardrail},
		Debug:           true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Testing guardrails...")
	response, err := ag.Run("Hello, how are you?")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println(response.TextContent)
}

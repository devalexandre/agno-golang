package main

import (
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/dashscope"
)

func main() {
	baseURL := os.Getenv("LLM_STUDIO_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("DASHSCOPE_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:1234/v1"
	}

	modelID := os.Getenv("LLM_STUDIO_MODEL")
	if modelID == "" {
		modelID = os.Getenv("DASHSCOPE_MODEL")
	}
	if modelID == "" {
		modelID = "qwen3-vl-2b-instruct"
	}

	model, err := dashscope.NewDashScopeChat(
		models.WithBaseURL(baseURL),
		models.WithID(modelID),
	)
	if err != nil {
		panic(err)
	}

	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:     "Qwen Local (LM Studio)",
		Model:    model,
		Markdown: true,
	})
	if err != nil {
		panic(err)
	}

	resp, err := agt.Run("Olá! Responda em 1 frase o que você consegue fazer.")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Model: %s\n", model.GetID())
	fmt.Printf("Response: %s\n", resp.TextContent)
}

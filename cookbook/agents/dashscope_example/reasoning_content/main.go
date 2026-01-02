package main

import (
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
		modelID = "qwen/qwen3-4b-thinking-2507"
	}

	options := []models.OptionClient{
		models.WithID(modelID),
		models.WithBaseURL(baseURL),
	}
	if os.Getenv("LLM_STUDIO_ENABLE_THINKING") == "1" {
		options = append(options, dashscope.WithEnableThinking(true))
	}

	model, err := dashscope.NewDashScopeChat(options...)
	if err != nil {
		panic(err)
	}

	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:     "Qwen Reasoning Content",
		Model:    model,
		Markdown: true,
	})
	if err != nil {
		panic(err)
	}

	agt.PrintResponse("Explique o que é o Qwen em 2 frases. Pense passo a passo antes de responder.", true, true)

}

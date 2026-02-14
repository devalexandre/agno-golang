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
		modelID = "qwen2.5-3b-instruct"
	}

	options := []models.OptionClient{
		models.WithID(modelID),
		models.WithBaseURL(baseURL),
	}

	model, err := dashscope.NewDashScopeChat(options...)
	if err != nil {
		panic(err)
	}

	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:          "DashScope Example",
		Model:         model,
		Markdown:      true,
		ShowToolsCall: false,
	})
	if err != nil {
		panic(err)
	}

	resp, err := agt.Run("Explique em 2 frases o que é o Qwen.")
	if err != nil {
		panic(err)
	}

	if len(resp.Messages) > 0 && resp.Messages[0].Thinking != "" {
		fmt.Printf("Thinking:\n%s\n\n", resp.Messages[0].Thinking)
	}
	fmt.Printf("Response: %s\n", resp.TextContent)
}

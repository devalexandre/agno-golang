package main

import (
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/flow"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("OPENAI_API_KEY not set")
		return
	}

	// 1. Setup Model
	model, err := chat.NewOpenAIChat()
	if err != nil {
		fmt.Printf("Error creating model: %v\n", err)
		return
	}

	// 2. Setup Agents
	researcher, err := agent.NewAgent(agent.AgentConfig{
		Name:         "Researcher",
		Model:        model,
		Instructions: "Search for facts about the given topic. Provide a concise summary.",
	})
	if err != nil {
		fmt.Printf("Error creating researcher agent: %v\n", err)
		return
	}

	writer, err := agent.NewAgent(agent.AgentConfig{
		Name:         "Writer",
		Model:        model,
		Instructions: "Write a professional email based on the research provided.",
	})
	if err != nil {
		fmt.Printf("Error creating writer agent: %v\n", err)
		return
	}

	// 3. Construct Workflow using Fluid API
	writerStep, _ := v2.NewStep(
		v2.WithName("writer"),
		v2.WithAgent(writer),
	)

	workflow := flow.New("AI Content Flow").
		Description("A flow that researches and writes an email").
		Step("research", researcher).
		If(flow.IfSuccess(),
			writerStep,
		).
		Else(
			func(input *v2.StepInput) (*v2.StepOutput, error) {
				return &v2.StepOutput{
					Content: "Research failed, skipping writer step.",
				}, nil
			},
		).
		Build()

	// 4. Run Workflow
	workflow.PrintResponse("The impact of AI on software engineering in 2026", true)
}

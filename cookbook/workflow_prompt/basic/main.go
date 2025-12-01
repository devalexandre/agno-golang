package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openrouter"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	// Create Ollama model
	ollamaModel, err := openrouter.NewOpenRouterChat(
		models.WithID("x-ai/grok-4.1-fast:free"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Create agents
	fmt.Println("👥 Creating agents...")
	fmt.Println("")

	researcher, err := agent.NewAgent(agent.AgentConfig{
		Name:   "Researcher",
		Model:  ollamaModel,
		Tools:  []toolkit.Tool{tools.NewDuckDuckGoTool()},
		Stream: true, // Enable streaming for the agent
	})
	if err != nil {
		fmt.Printf("Erro ao criar agente Researcher: %v\n", err)
		return
	}

	writer, err := agent.NewAgent(agent.AgentConfig{
		Name:         "Writer",
		Model:        ollamaModel,
		Instructions: "Write engaging content",
		Stream:       true, // Enable streaming for the agent
	})
	if err != nil {
		fmt.Printf("Erro ao criar agente Writer: %v\n", err)
		return
	}

	// Crie steps usando NewStep e WithAgent
	researcherStep, err := v2.NewStep(
		v2.WithName("Researcher"),
		v2.WithAgent(researcher),
		v2.WithStepStreaming(true), // Enable streaming for the step
	)
	if err != nil {
		fmt.Printf("Erro ao criar step Researcher: %v\n", err)
		return
	}

	writerStep, err := v2.NewStep(
		v2.WithName("Writer"),
		v2.WithAgent(writer),
		v2.WithStepStreaming(true), // Enable streaming for the step
	)
	if err != nil {
		fmt.Printf("Erro ao criar step Writer: %v\n", err)
		return
	}

	workflow := v2.NewWorkflow(
		v2.WithWorkflowName("Content Workflow"),
		v2.WithWorkflowDescription("A workflow for creating content"),
		v2.WithWorkflowSteps([]*v2.Step{researcherStep, writerStep}),
		v2.WithStreaming(true, true),
	)

	input := "Create a blog post about AI agents"
	workflow.PrintResponse(input, true)
}

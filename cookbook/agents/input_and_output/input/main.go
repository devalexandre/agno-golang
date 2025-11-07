package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

// ResearchTopic represents a structured research topic input
type ResearchTopic struct {
	Topic           string   `json:"topic" description:"The main research topic"`
	FocusAreas      []string `json:"focus_areas" description:"Specific areas to focus on"`
	TargetAudience  string   `json:"target_audience" description:"Who this research is for"`
	SourcesRequired int      `json:"sources_required" description:"Number of sources needed"`
}

func main() {
	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Agent with input_schema
	researchAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:     context.Background(),
		Model:       model,
		Name:        "Research Agent",
		Role:        "Extract key insights and content from research topics",
		InputSchema: ResearchTopic{},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Input Schema Example ===")
	fmt.Println("Using input_schema to validate and structure input")
	fmt.Println()

	// Create input as struct (like Python: input=ResearchTopic(...))
	topic := ResearchTopic{
		Topic:           "AI",
		FocusAreas:      []string{"Machine Learning", "Deep Learning"},
		TargetAudience:  "Developers",
		SourcesRequired: 5,
	}

	fmt.Printf("Input Topic: %s\n", topic.Topic)
	fmt.Printf("Focus Areas: %v\n", topic.FocusAreas)
	fmt.Printf("Target Audience: %s\n", topic.TargetAudience)
	fmt.Printf("Sources Required: %d\n\n", topic.SourcesRequired)

	// Run agent with structured input directly (no need to marshal)
	run, err := researchAgent.Run(topic)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	fmt.Println("Agent Response:")
	fmt.Println(run.TextContent)
}

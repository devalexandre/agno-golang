package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

// ResearchTopic defines the input schema
type ResearchTopic struct {
	Topic           string `json:"topic" description:"The main research topic"`
	SourcesRequired int    `json:"sources_required" description:"Number of sources needed"`
}

// ResearchOutput defines the output schema
type ResearchOutput struct {
	Summary      string   `json:"summary" description:"Executive summary of the research"`
	Insights     []string `json:"insights" description:"Key insights from the topic"`
	TopStories   []string `json:"top_stories" description:"Most relevant and popular stories"`
	Technologies []string `json:"technologies" description:"Technologies mentioned"`
	Sources      []string `json:"sources" description:"Links or references to relevant sources"`
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

	// Create agent with both input and output schemas
	researchAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Model:         model,
		Name:          "Research Agent",
		Role:          "Technical Research Specialist",
		Instructions:  "Research topics and provide comprehensive insights with sources",
		InputSchema:   ResearchTopic{},
		OutputSchema:  ResearchOutput{},
		ParseResponse: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Create input as struct (like Python: input=ResearchTopic(...))
	topic := ResearchTopic{
		Topic:           "AI and Machine Learning",
		SourcesRequired: 5,
	}

	fmt.Println("=== Research Agent with Input and Output Schemas ===")
	fmt.Printf("Researching: %s\n\n", topic.Topic)

	// Run agent with structured input (no need to marshal)
	run, err := researchAgent.Run(topic)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	// Access parsed output (like Python: response.content)
	if run.ParsedOutput != nil {
		result := run.ParsedOutput.(*ResearchOutput)

		fmt.Println("Summary:")
		fmt.Println(result.Summary)
		fmt.Println()

		fmt.Println("Key Insights:")
		for i, insight := range result.Insights {
			fmt.Printf("%d. %s\n", i+1, insight)
		}
		fmt.Println()

		fmt.Println("Top Stories:")
		for i, story := range result.TopStories {
			fmt.Printf("%d. %s\n", i+1, story)
		}
		fmt.Println()

		fmt.Println("Technologies Mentioned:")
		for i, tech := range result.Technologies {
			fmt.Printf("%d. %s\n", i+1, tech)
		}
		fmt.Println()

		fmt.Println("Sources:")
		for i, source := range result.Sources {
			fmt.Printf("%d. %s\n", i+1, source)
		}
	} else {
		// Fallback to raw text content
		fmt.Println("Raw response:")
		fmt.Println(run.TextContent)
	}
}

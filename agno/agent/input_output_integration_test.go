package agent

import (
	"context"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

// Test structs (same as examples)
type MovieScript struct {
	Setting    string   `json:"setting" description:"Provide a nice setting for a blockbuster movie."`
	Ending     string   `json:"ending" description:"Ending of the movie. If not available, provide a happy ending."`
	Genre      string   `json:"genre" description:"Genre of the movie. If not available, select action, thriller or romantic comedy."`
	Name       string   `json:"name" description:"Give a name to this movie"`
	Characters []string `json:"characters" description:"Name of characters for this movie."`
	Storyline  string   `json:"storyline" description:"3 sentence storyline for the movie. Make it exciting!"`
}

type ResearchTopic struct {
	Topic           string   `json:"topic" description:"The main research topic"`
	FocusAreas      []string `json:"focus_areas" description:"Specific areas to focus on"`
	TargetAudience  string   `json:"target_audience" description:"Who this research is for"`
	SourcesRequired int      `json:"sources_required" description:"Number of sources needed"`
}

type ResearchOutput struct {
	Summary      string   `json:"summary" description:"Executive summary of the research"`
	Insights     []string `json:"insights" description:"Key insights from the topic"`
	TopStories   []string `json:"top_stories" description:"Most relevant and popular stories"`
	Technologies []string `json:"technologies" description:"Technologies mentioned"`
	Sources      []string `json:"sources" description:"Links or references to relevant sources"`
}

// TestOutputSchemaObject tests output schema with a single MovieScript object
// Based on: examples/input-output/output/main.go
func TestOutputSchemaObject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with Ollama in short mode")
	}

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		t.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create a pointer to MovieScript - it will be filled automatically after Run()
	movieScript := &MovieScript{}

	// Agent that uses structured outputs
	agent, err := NewAgent(AgentConfig{
		Context:       context.Background(),
		Model:         model,
		Description:   "You write movie scripts.",
		OutputSchema:  movieScript,
		ParseResponse: true,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Run agent
	run, err := agent.Run("Create a movie script set in New York")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify the original pointer is filled
	if movieScript.Name == "" {
		t.Error("Expected movie name to be filled")
	}
	if movieScript.Setting == "" {
		t.Error("Expected movie setting to be filled")
	}
	if len(movieScript.Characters) == 0 {
		t.Error("Expected at least one character")
	}

	// Verify run.Output points to the same data
	if run.Output == nil {
		t.Fatal("run.Output is nil")
	}

	outputScript := run.Output.(*MovieScript)
	if outputScript != movieScript {
		t.Error("run.Output should point to the same movieScript instance")
	}

	t.Logf("Generated movie: %s (%s)", movieScript.Name, movieScript.Genre)
}

// TestOutputSchemaSlice tests output schema with a slice of MovieScript
// Based on: examples/input-output/output-slice/main.go
func TestOutputSchemaSlice(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with Ollama in short mode")
	}

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		t.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create a pointer to slice - it will be filled automatically after Run()
	movieScripts := &[]MovieScript{}

	// Agent that uses structured outputs with slice
	agent, err := NewAgent(AgentConfig{
		Context:       context.Background(),
		Model:         model,
		Description:   "You write movie scripts.",
		OutputSchema:  movieScripts,
		ParseResponse: true,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Run agent
	run, err := agent.Run("Create 3 different movie scripts: one set in New York, one in Tokyo, and one in Paris")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify the original pointer is filled
	if len(*movieScripts) == 0 {
		t.Fatal("Expected at least one movie script")
	}

	// Ideally should be 3, but LLM might return different number
	t.Logf("Generated %d movie scripts", len(*movieScripts))

	// Check first movie has required fields
	if (*movieScripts)[0].Name == "" {
		t.Error("First movie should have a name")
	}

	// Verify run.Output points to the same data
	if run.Output == nil {
		t.Fatal("run.Output is nil")
	}

	outputScripts := run.Output.(*[]MovieScript)
	if outputScripts != movieScripts {
		t.Error("run.Output should point to the same movieScripts slice")
	}
}

// TestInputSchema tests input schema with ResearchTopic
// Based on: examples/input-output/input/main.go
func TestInputSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with Ollama in short mode")
	}

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		t.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Agent with input schema
	agent, err := NewAgent(AgentConfig{
		Context:     context.Background(),
		Model:       model,
		Name:        "Research Agent",
		Role:        "Extract key insights and content from research topics",
		InputSchema: ResearchTopic{},
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Create input as struct
	topic := ResearchTopic{
		Topic:           "Artificial Intelligence",
		FocusAreas:      []string{"Machine Learning", "Deep Learning"},
		TargetAudience:  "Developers",
		SourcesRequired: 5,
	}

	// Run agent with structured input (no need to marshal)
	run, err := agent.Run(topic)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify response was generated
	if run.TextContent == "" {
		t.Error("Expected non-empty response content")
	}

	t.Logf("Response length: %d characters", len(run.TextContent))
}

// TestInputAndOutputSchema tests using both input and output schemas together
// Based on: examples/input-output/both/main.go
func TestInputAndOutputSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test with Ollama in short mode")
	}

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		t.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create output pointer
	researchOutput := &ResearchOutput{}

	// Create agent with both input and output schemas
	agent, err := NewAgent(AgentConfig{
		Context:       context.Background(),
		Model:         model,
		Name:          "Research Agent",
		Role:          "Technical Research Specialist",
		Instructions:  "Research topics and provide comprehensive insights with sources",
		InputSchema:   ResearchTopic{},
		OutputSchema:  researchOutput,
		ParseResponse: true,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Create input as struct
	topic := ResearchTopic{
		Topic:           "Go Programming Language",
		FocusAreas:      []string{"Concurrency", "Performance"},
		TargetAudience:  "Backend Developers",
		SourcesRequired: 3,
	}

	// Run agent with structured input
	run, err := agent.Run(topic)
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify output is filled
	if researchOutput.Summary == "" {
		t.Error("Expected summary to be filled")
	}

	// Verify run.Output
	if run.Output == nil {
		t.Fatal("run.Output is nil")
	}

	outputResearch := run.Output.(*ResearchOutput)
	if outputResearch != researchOutput {
		t.Error("run.Output should point to the same researchOutput instance")
	}

	t.Logf("Summary length: %d characters", len(researchOutput.Summary))
	t.Logf("Number of insights: %d", len(researchOutput.Insights))
}

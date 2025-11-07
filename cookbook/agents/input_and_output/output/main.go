package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

// MovieScript represents a structured movie script output
type MovieScript struct {
	Setting    string   `json:"setting" description:"Provide a nice setting for a blockbuster movie."`
	Ending     string   `json:"ending" description:"Ending of the movie. If not available, provide a happy ending."`
	Genre      string   `json:"genre" description:"Genre of the movie. If not available, select action, thriller or romantic comedy."`
	Name       string   `json:"name" description:"Give a name to this movie"`
	Characters []string `json:"characters" description:"Name of characters for this movie."`
	Storyline  string   `json:"storyline" description:"3 sentence storyline for the movie. Make it exciting!"`
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

	// Create a pointer to MovieScript - it will be filled automatically after Run()
	movieScript := &MovieScript{}

	// Agent that uses structured outputs
	structuredOutputAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Model:         model,
		Description:   "You write movie scripts.",
		OutputSchema:  movieScript, // Pass pointer here
		ParseResponse: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Output Schema Example ===")
	fmt.Println("Using output_schema to get structured movie script")
	fmt.Println()

	// Get the response
	run, err := structuredOutputAgent.Run("Create a movie script set in New York")
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	// Two ways to access the result:

	// 1. Use the original pointer (movieScript is already filled!)
	fmt.Println("Method 1: Using original pointer")
	scriptJSON, _ := json.MarshalIndent(movieScript, "", "  ")
	fmt.Println(string(scriptJSON))

	fmt.Printf("\nDirect access to fields:\n")
	fmt.Printf("Movie Name: %s\n", movieScript.Name)
	fmt.Printf("Genre: %s\n", movieScript.Genre)
	fmt.Printf("Setting: %s\n", movieScript.Setting)

	// 2. Use run.Output (points to the same data)
	fmt.Println("\n\nMethod 2: Using run.Output")
	outputScript := run.Output.(*MovieScript)
	fmt.Printf("Movie Name: %s\n", outputScript.Name)
	fmt.Printf("Same pointer? %v\n", movieScript == outputScript)
}

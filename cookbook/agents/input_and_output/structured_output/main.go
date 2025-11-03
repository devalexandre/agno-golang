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

	// Agent that uses structured outputs
	structuredOutputAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:       context.Background(),
		Model:         model,
		Description:   "You write movie scripts.",
		OutputSchema:  MovieScript{},
		ParseResponse: true, // Enable automatic parsing
		Debug:         true, // Enable debug to see what's happening
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Get the response with a more detailed prompt
	run, err := structuredOutputAgent.Run("Create a movie script set in New York. Include a name, genre, setting, characters, storyline and ending.")
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	// The ParsedOutput contains the structured MovieScript
	if run.ParsedOutput != nil {
		movieScript := run.ParsedOutput.(*MovieScript)

		// Pretty print the structured output
		scriptJSON, _ := json.MarshalIndent(movieScript, "", "  ")
		fmt.Println(string(scriptJSON))
	}
}

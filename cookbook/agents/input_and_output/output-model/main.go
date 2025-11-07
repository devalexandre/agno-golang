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
	ctx := context.Background()

	// Create main model for content generation
	// This could be a more powerful/expensive model
	mainModel, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create main model: %v", err)
	}

	// Create output model for JSON formatting
	// This could be a faster/cheaper model specifically good at structured output
	outputModel, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"), // In production, you might use a different model
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create output model: %v", err)
	}

	// Create a pointer to MovieScript - it will be filled automatically
	movieScript := &MovieScript{}

	// Agent using OutputModel for two-stage processing:
	// 1. Main model generates creative content freely (no schema constraints)
	// 2. Output model formats the content into structured JSON
	agentWithOutputModel, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Model:         mainModel,
		OutputModel:   outputModel, // Separate model for JSON formatting
		Description:   "You are a creative movie script writer. Focus on creating engaging content.",
		OutputSchema:  movieScript,
		ParseResponse: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== OutputModel Example ===")
	fmt.Println("Demonstrates TWO-STAGE processing:")
	fmt.Println("1. Main model (expensive) → generates creative content with simple prompt")
	fmt.Println("2. Output model (cheap) → formats content into structured JSON")
	fmt.Println()
	fmt.Println("Benefits:")
	fmt.Println("- Use expensive model only for creative work (shorter prompt = cheaper)")
	fmt.Println("- Use cheap model for mechanical JSON formatting")
	fmt.Println("- Get BOTH outputs: original creative text + structured data")
	fmt.Println()

	// Run the agent with simple prompt
	// Main model receives: "Create a sci-fi movie about AI"
	// OutputModel receives: main model's response + schema instructions
	response, err := agentWithOutputModel.Run("Create a sci-fi movie about AI")
	if err != nil {
		log.Fatalf("Failed to run agent: %v", err)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("OUTPUT 1: Original Creative Text (from Main Model)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(response.TextContent)
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("OUTPUT 2: Structured JSON (formatted by Output Model)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	// movieScript pointer is now filled with structured data
	movieJSON, _ := json.MarshalIndent(movieScript, "", "  ")
	fmt.Println(string(movieJSON))
	fmt.Println()

	// You can also access through response.Output
	if response.Output != nil {
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("Access Methods for Structured Data")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("Method 1: Direct pointer access (movieScript)")
		fmt.Printf("  Movie Name: %s\n", movieScript.Name)
		fmt.Printf("  Genre: %s\n", movieScript.Genre)
		fmt.Println()

		fmt.Println("Method 2: Via response.Output")
		if script, ok := response.Output.(*MovieScript); ok {
			fmt.Printf("  Movie Name: %s\n", script.Name)
			fmt.Printf("  Setting: %s\n", script.Setting)
			fmt.Printf("  Characters: %v\n", script.Characters)
			fmt.Printf("  Same pointer? %v\n", movieScript == script)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Custom OutputModelPrompt Example")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Using custom formatting instructions for OutputModel")
	fmt.Println()

	// Create another agent with custom output model prompt
	movieScript2 := &MovieScript{}
	customPrompt := `You are a JSON formatter. Convert the creative text into a strict JSON structure.
Be extremely concise in your JSON values - use short, punchy descriptions.

Return ONLY valid JSON matching the schema. No explanations, no markdown.`

	agentWithCustomPrompt, err := agent.NewAgent(agent.AgentConfig{
		Context:           ctx,
		Model:             mainModel,
		OutputModel:       outputModel,
		OutputModelPrompt: customPrompt, // Custom instructions for formatting
		Description:       "You write epic fantasy movie scripts.",
		OutputSchema:      movieScript2,
		ParseResponse:     true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent with custom prompt: %v", err)
	}

	_, err = agentWithCustomPrompt.Run("Create a fantasy movie about dragons")
	if err != nil {
		log.Fatalf("Failed to run agent: %v", err)
	}

	fmt.Println("--- Structured Output (with custom prompt) ---")
	movieJSON2, _ := json.MarshalIndent(movieScript2, "", "  ")
	fmt.Println(string(movieJSON2))
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/utils"
)

// MovieRecommendation represents a structured movie recommendation
type MovieRecommendation struct {
	Title       string   `json:"title"`
	Year        int      `json:"year"`
	Genre       []string `json:"genre"`
	Director    string   `json:"director"`
	Rating      float64  `json:"rating"`
	Description string   `json:"description"`
	WhyWatch    string   `json:"why_watch"`
}

func main() {
	ctx := context.Background()

	// Enable markdown
	utils.SetMarkdownMode(true)

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create agent
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "MovieExpert",
		Instructions: `You are a passionate movie expert and critic! 🎬
You have extensive knowledge of cinema across all genres and eras.

When recommending movies:
1. Consider the user's preferences and mood
2. Provide diverse recommendations across different genres
3. Include both classics and modern films
4. Explain why each movie is worth watching
5. Be enthusiastic and engaging in your descriptions

IMPORTANT: Format your response as a JSON array of movie recommendations with this structure:
[
  {
    "title": "Movie Title",
    "year": 2023,
    "genre": ["Genre1", "Genre2"],
    "director": "Director Name",
    "rating": 8.5,
    "description": "Brief description",
    "why_watch": "Why this movie is recommended"
  }
]`,
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example usage - request structured output
	start := time.Now()
	response, err := ag.Run("Recommend 3 sci-fi movies for a weekend marathon. Return as JSON array.")
	if err != nil {
		log.Fatal(err)
	}

	// Try to parse the response as JSON
	var recommendations []MovieRecommendation
	if err := json.Unmarshal([]byte(response.TextContent), &recommendations); err != nil {
		// If parsing fails, just show the text response
		utils.ResponsePanel(response.TextContent, nil, start, true)
	} else {
		// Successfully parsed structured output
		utils.SuccessPanel("Received structured movie recommendations!")

		for i, rec := range recommendations {
			panel := fmt.Sprintf(`**Movie %d: %s (%d)**

**Genre:** %v
**Director:** %s
**Rating:** %.1f/10

**Description:**
%s

**Why watch:**
%s`,
				i+1, rec.Title, rec.Year, rec.Genre, rec.Director, rec.Rating, rec.Description, rec.WhyWatch)

			utils.InfoPanel(panel)
		}
	}

	// Alternative: Use PrintResponse for conversational output
	// ag.PrintResponse("Recommend 3 sci-fi movies for a weekend marathon", true, true)
}

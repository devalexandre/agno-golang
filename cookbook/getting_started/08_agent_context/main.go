package main

import (
	"context"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/utils"
)

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

	// Create agent with rich context
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "TravelAdvisor",
		Instructions: `You are an expert travel advisor! ✈️

Use the provided context about the user to give personalized travel recommendations.

Consider:
- User's budget and preferences
- Travel dates and duration
- Interests and activities
- Dietary restrictions
- Previous travel experiences

Provide detailed, personalized advice that matches the user's profile.`,
		AddDatetimeToContext: true, // Add current date/time to context
		AddNameToContext:     true, // Add agent name to context
		TimezoneIdentifier:   "America/Sao_Paulo",
		ContextData: map[string]interface{}{
			"user_profile": map[string]interface{}{
				"name":        "Alex",
				"age":         30,
				"nationality": "Brazilian",
				"languages":   []string{"Portuguese", "English", "Spanish"},
			},
			"preferences": map[string]interface{}{
				"budget":            "moderate",
				"travel_style":      "adventure",
				"interests":         []string{"hiking", "photography", "local_cuisine"},
				"dietary":           "vegetarian",
				"accommodation":     "boutique_hotels",
				"previous_trips":    []string{"Peru", "Argentina", "Portugal"},
				"dream_destination": "Japan",
			},
			"constraints": map[string]interface{}{
				"vacation_days": 15,
				"season":        "spring",
				"companions":    "solo",
			},
		},
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example usage - agent will use all the context
	ag.PrintResponse("I have 2 weeks off in March. Where should I travel and what should I do?", true, true)

	time.Sleep(1 * time.Second)

	// Follow-up question - context is maintained
	utils.InfoPanel("\nFollow-up question:")
	ag.PrintResponse("What about food recommendations for this trip?", true, true)
}

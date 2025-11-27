package main

import (
	"context"
	"fmt"
	"log"

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

	// Create agent with state management
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "GameMaster",
		Instructions: `You are a game master for a text-based adventure game! 🎮

You manage the game state and guide players through an adventure.

Game mechanics:
- Track player's inventory, health, and location
- Update state based on player actions
- Provide engaging narrative responses
- Maintain consistency with the current game state

Always describe the current state and available actions to the player.`,
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Initialize game state
	sessionID := "game_session_1"
	gameState := map[string]interface{}{
		"location":  "forest_entrance",
		"health":    100,
		"inventory": []string{"sword", "torch"},
		"gold":      50,
	}

	// First action
	utils.InfoPanel("Game Start:")
	response1, err := ag.Run(
		"I want to explore the forest. What do I see?",
		agent.WithSessionID(sessionID),
		agent.WithSessionState(gameState),
		agent.WithAddSessionStateToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🎮 %s\n\n", response1.TextContent)

	// Update state after action
	gameState["location"] = "deep_forest"
	gameState["inventory"] = []string{"sword", "torch", "healing_potion"}

	// Second action with updated state
	utils.InfoPanel("After exploring:")
	response2, err := ag.Run(
		"What's in my inventory and where am I now?",
		agent.WithSessionID(sessionID),
		agent.WithSessionState(gameState),
		agent.WithAddSessionStateToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🎮 %s\n\n", response2.TextContent)

	// Show final state
	utils.SuccessPanel(fmt.Sprintf(`Final Game State:
Location: %v
Health: %v
Inventory: %v
Gold: %v`,
		gameState["location"],
		gameState["health"],
		gameState["inventory"],
		gameState["gold"]))
}

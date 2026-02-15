package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/together"
)

func main() {
	ctx := context.Background()

	// Create Together AI model
	model, err := together.NewTogetherChat(
		models.WithID(together.ModelLlama318BInstruct),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Create agent with ALL built-in skills
	//
	// TWO-STAGE PROCESS:
	// 1. LOADING: ALL built-in skills from ./skills are loaded automatically
	//    (github, slack, discord, notion, trello, weather, summarize, obsidian, etc.)
	//
	// 2. ACTIVATION: Use SkillsUseAll = true to activate ALL loaded skills
	//
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Universal Assistant",
		Instructions: "You are a universal assistant with access to all available skills.",

		// STAGE 2: Use SkillsUseAll to activate ALL loaded skills
		SkillsUseAll: true, // Activates ALL skills (overrides SkillsToUse)
		//
		// Alternative (same result):
		// Don't set SkillsUseAll and don't set SkillsToUse - defaults to all active
		//
		// Available skills:
		// ✓ github, slack, discord, notion, trello, weather,
		// ✓ summarize, obsidian, coding-agent, skill-creator
		// ✓ ... all built-in skills

		Markdown:      true,
		ShowToolsCall: true,
		ShowSkillCall: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// RESULT:
	// - 10+ skills loaded in memory (STAGE 1)
	// - ALL skills active (STAGE 2 - not specified)
	// - Agent can access ALL built-in skills
	a.PrintResponse("What skills do you have available?", false, true)
}

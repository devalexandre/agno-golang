package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/together"
	"github.com/devalexandre/agno-golang/agno/skill"
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

	// Load custom skills from a directory
	customLoader := skill.NewLocalSkills("./my-custom-skills")

	// Create agent with both built-in and custom skills
	//
	// TWO-STAGE PROCESS:
	// 1. LOADING:
	//    - ALL built-in skills from ./skills are loaded automatically
	//    - ALL custom skills from ./my-custom-skills are loaded
	//
	// 2. ACTIVATION:
	//    - SkillsToUse controls which built-in skills are active
	//    - Custom skills are ALWAYS active (all loaded custom skills)
	//
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Full Assistant",
		Instructions: "You are a versatile assistant with many skills.",

		// STAGE 2: Activate specific built-in skills
		SkillsToUse: []string{
			"github",  // ✓ Active built-in skill
			"slack",   // ✓ Active built-in skill
			"weather", // ✓ Active built-in skill
			// discord, notion, trello, etc. are loaded but inactive
		},

		// STAGE 1: Load custom skills (all will be active)
		CustomSkillsLoader: customLoader,

		Markdown:      true,
		ShowToolsCall: true,
		ShowSkillCall: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// RESULT:
	// STAGE 1 - Loaded:
	//   - 10+ built-in skills from ./skills
	//   - N custom skills from ./my-custom-skills
	//
	// STAGE 2 - Active:
	//   - 3 built-in skills: github, slack, weather
	//   - ALL custom skills (always active)
	a.PrintResponse("What's the weather like today?", false, true)
}

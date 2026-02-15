package main

import (
	"context"
	"log"
	"os"

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
		// API key will be read from TOGETHER_API_KEY environment variable
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Load custom skills from sample_skills directory
	// Try both relative paths to support running from different directories
	customSkillsPath := "../sample_skills"
	if _, err := os.Stat(customSkillsPath); os.IsNotExist(err) {
		customSkillsPath = "./cookbook/agents/skills/sample_skills"
	}
	customLoader := skill.NewLocalSkills(customSkillsPath)

	// Create agent with specific skills
	//
	// TWO-STAGE PROCESS:
	// 1. LOADING:
	//    - ALL built-in skills from ./skills are loaded automatically
	//      (github, slack, discord, notion, trello, weather, summarize, obsidian, coding-agent, etc.)
	//    - ALL custom skills from sample_skills are loaded
	//      (code-review, git-workflow)
	//
	// 2. ACTIVATION: Only skills in SkillsToUse are accessible to the agent
	//    All loaded skills exist in memory, but agent can only use the specified ones
	//
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Code Assistant",
		Instructions: "You are a helpful coding assistant.",

		// STAGE 2: Specify which skills the agent can USE
		// (All built-in and custom skills are already LOADED in stage 1)
		SkillsToUse: []string{
			"code-review", // ✓ Active - custom skill from sample_skills
			"github",      // ✓ Active - built-in skill
			// slack      // ✗ Loaded but inactive - agent cannot use
			// discord    // ✗ Loaded but inactive - agent cannot use
			// ... other skills are loaded but not accessible
		},

		// STAGE 1: Load custom skills
		CustomSkillsLoader: customLoader,

		Markdown:      true,
		ShowToolsCall: true,
		ShowSkillCall: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// RESULT:
	// - 10+ built-in skills + 2 custom skills loaded in memory (STAGE 1)
	// - Only 2 skills active: code-review (custom), github (built-in) (STAGE 2)
	// - Agent can only access the 2 active skills
	a.PrintResponse("Review this code for style issues: func foo() { fmt.Println(\"hello\") }", false, true)

}

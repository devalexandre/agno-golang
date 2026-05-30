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

	customSkillsPath := "../sample_skills"
	if _, err := os.Stat(customSkillsPath); os.IsNotExist(err) {
		customSkillsPath = "./cookbook/agents/skills/sample_skills"
	}
	customLoader := skill.NewLocalSkills(customSkillsPath)

	a, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Model:        model,
		Name:         "Code Assistant",
		Instructions: "You are a helpful coding assistant.",

		SkillsToUse: []string{
			"code-review", // ✓ Active - custom skill from sample_skills
			"github",      // ✓ Active - built-in skill
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
	a.PrintResponse("Review this code for style issues: func foo() { fmt.Println(\"hello\") }", false, true)

}

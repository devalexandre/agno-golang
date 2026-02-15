package main

// ⚠️  ADVANCED EXAMPLE - DEPRECATED APPROACH
//
// This example shows the OLD way of managing skills using manual loaders and WithFilter.
// This approach is MORE COMPLEX and NOT RECOMMENDED for most use cases.
//
// RECOMMENDED APPROACH (simpler):
//   Use SkillsToUse in AgentConfig instead - see ../simple-usage/ example
//
// KEY DIFFERENCE:
//   - This approach: Filters at LOADING time (only filtered skills are loaded)
//   - Recommended approach: Loads ALL, filters at ACTIVATION time (two-stage process)
//
// Use this approach only when:
//   - You need fine-grained control over which skills are loaded into memory
//   - You're working with a very large number of skills and want to minimize memory usage
//   - You need to combine multiple skill sources with different filters

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/together"
	"github.com/devalexandre/agno-golang/agno/skill"
)

func main() {
	ctx := context.Background()

	// ADVANCED: Manual skill loading with filtering
	//
	// WithFilter loads ONLY the specified skills (filters at load time)
	// This is different from SkillsToUse which loads ALL but activates selected ones
	//
	builtinLoader := skill.NewLocalSkills(
		"../../../skills",
		skill.WithFilter([]string{"code-review", "github"}),
	)

	userLoader := skill.NewLocalSkills("./my-custom-skills")

	// Manually create Skills orchestrator
	// Only code-review and github will be loaded from built-in
	skills, err := skill.NewSkills(builtinLoader, userLoader)
	if err != nil {
		log.Fatalf("Failed to load skills: %v", err)
	}

	// List loaded skills
	fmt.Println("Loaded skills:")
	for _, s := range skills.GetAllSkills() {
		fmt.Printf("  - %s: %s\n", s.Name, s.Description)
	}

	// Create Together AI model
	model, err := together.NewTogetherChat(
		models.WithID(together.ModelLlama318BInstruct),
		models.WithAPIKey("your-api-key"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Create agent with filtered skills
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Model:         model,
		Name:          "Selective Assistant",
		Instructions:  "You are a helpful assistant with specific skills.",
		Skills:        skills,
		Markdown:      true,
		ShowToolsCall: true,
		ShowSkillCall: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Use the agent
	a.PrintResponse("Review my code and help me create a PR", false, true)
}

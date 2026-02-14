package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/together"
	"github.com/devalexandre/agno-golang/agno/skill"
)

func main() {
	ctx := context.Background()

	// Determine path to sample skills relative to this file
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	skillsPath := filepath.Join(dir, "sample_skills")

	fmt.Printf("Loading skills from: %s\n\n", skillsPath)

	// Create a local skills loader
	loader := skill.NewLocalSkills(skillsPath, skill.WithValidation(true))

	// Create the Skills orchestrator
	skills, err := skill.NewSkills(loader)
	if err != nil {
		log.Fatalf("Failed to load skills: %v", err)
	}

	// Create Together AI model (better tool calling support)
	model, err := together.NewTogetherChat(
		models.WithID(together.ModelLlama318BInstruct),
		models.WithAPIKey("your-api-key"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Create agent with skills
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Model:         model,
		Name: "Code Assistant",
		Instructions: `You are a helpful coding assistant with access to specialized skills.

When you receive a task that matches one of your skills, you MUST:
1. Call Skills_GetInstructions to load the skill's guidance
2. Follow the skill's instructions to complete the task
3. Use Skills_GetReference or Skills_GetScript as needed

DO NOT just describe what tools you could use - actually USE them.`,
		Skills:        skills,
		Markdown:      true,
		ShowToolsCall: true,
		ShowSkillCall: true,
		Debug:         false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Run the agent with a prompt that should trigger skill usage
	fmt.Println("Running agent with skills...")
	fmt.Println("---")
	// resp, err := a.Run("Please review the following Go code for style issues:\n\nfunc processData(d []byte) { fmt.Println(string(d)) }")
	// if err != nil {
	// 	log.Fatalf("Run failed: %v", err)
	// }

	// fmt.Println(resp.TextContent)
	a.PrintResponse("Please review the following Go code for style issues:\n\nfunc processData(d []byte) { fmt.Println(string(d)) }", false, true)
}

package skill

import (
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Skills orchestrates skill loading and provides tools for agents to access skills.
//
// The Skills struct is responsible for:
//  1. Loading skills from various sources (loaders)
//  2. Providing methods to access loaded skills
//  3. Generating tools for agents to use skills
//  4. Creating system prompt snippets with available skills metadata
type Skills struct {
	loaders []SkillLoader
	skills  map[string]Skill
}

// NewSkills creates a new Skills orchestrator with the given loaders.
// It immediately loads all skills from the loaders.
func NewSkills(loaders ...SkillLoader) (*Skills, error) {
	s := &Skills{
		loaders: loaders,
		skills:  make(map[string]Skill),
	}

	if err := s.loadSkills(); err != nil {
		return nil, err
	}

	return s, nil
}

// loadSkills loads skills from all loaders.
func (s *Skills) loadSkills() error {
	for _, loader := range s.loaders {
		skills, err := loader.Load()
		if err != nil {
			// Check if it's a validation error - those are hard failures
			if _, ok := err.(*SkillValidationError); ok {
				return err
			}
			log.Printf("Error loading skills from loader: %v", err)
			continue
		}

		for _, sk := range skills {
			if _, exists := s.skills[sk.Name]; exists {
				log.Printf("Duplicate skill name '%s', overwriting with newer version", sk.Name)
			}
			s.skills[sk.Name] = sk
		}
	}

	log.Printf("Loaded %d total skills", len(s.skills))
	return nil
}

// Reload clears and reloads skills from all loaders.
func (s *Skills) Reload() error {
	s.skills = make(map[string]Skill)
	return s.loadSkills()
}

// GetSkill returns a skill by name.
func (s *Skills) GetSkill(name string) (*Skill, bool) {
	sk, ok := s.skills[name]
	if !ok {
		return nil, false
	}
	return &sk, true
}

// GetAllSkills returns all loaded skills.
func (s *Skills) GetAllSkills() []Skill {
	result := make([]Skill, 0, len(s.skills))
	for _, sk := range s.skills {
		result = append(result, sk)
	}
	return result
}

// GetSkillNames returns the names of all loaded skills.
func (s *Skills) GetSkillNames() []string {
	result := make([]string, 0, len(s.skills))
	for name := range s.skills {
		result = append(result, name)
	}
	return result
}

// GetSystemPromptSnippet generates a system prompt snippet with available skills metadata.
// This creates an XML-formatted snippet that provides the agent with
// information about available skills without including the full instructions.
func (s *Skills) GetSystemPromptSnippet() string {
	if len(s.skills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines,
		"<skills_system>",
		"",
		"## What are Skills?",
		"Skills are packages of domain expertise that extend your capabilities. Each skill contains:",
		"- **Instructions**: Detailed guidance on when and how to apply the skill",
		"- **Scripts**: Executable code templates you can use or adapt",
		"- **References**: Supporting documentation (guides, cheatsheets, examples)",
		"",
		"## IMPORTANT: How to Use Skills",
		"**Skill names are NOT callable functions.** You cannot call a skill directly by its name.",
		"Instead, you MUST use the provided skill access tools:",
		"",
		"1. `Skills_GetInstructions(skill_name)` - Load the full instructions for a skill",
		"2. `Skills_GetReference(skill_name, reference_path)` - Access specific documentation",
		"3. `Skills_GetScript(skill_name, script_path, execute=false)` - Read or run scripts",
		"",
		"## Progressive Discovery Workflow",
		"1. **Browse**: Review the skill summaries below to understand what's available",
		"2. **Load**: When a task matches a skill, call `Skills_GetInstructions(skill_name)` first",
		"3. **Reference**: Use `Skills_GetReference` to access specific documentation as needed",
		"4. **Scripts**: Use `Skills_GetScript` to read or execute scripts from a skill",
		"",
		"**IMPORTANT**: References are documentation files (NOT executable). Only use `Skills_GetScript` when `<scripts>` lists actual script files. If `<scripts>none</scripts>`, do NOT call `Skills_GetScript`.",
		"",
		"This approach ensures you only load detailed instructions when actually needed.",
		"",
		"## Available Skills",
	)

	for _, sk := range s.skills {
		lines = append(lines, "<skill>")
		lines = append(lines, fmt.Sprintf("  <name>%s</name>", sk.Name))
		lines = append(lines, fmt.Sprintf("  <description>%s</description>", sk.Description))

		if len(sk.Scripts) > 0 {
			lines = append(lines, fmt.Sprintf("  <scripts>%s</scripts>", strings.Join(sk.Scripts, ", ")))
		} else {
			lines = append(lines, "  <scripts>none</scripts>")
		}

		if len(sk.References) > 0 {
			lines = append(lines, fmt.Sprintf("  <references>%s</references>", strings.Join(sk.References, ", ")))
		}

		lines = append(lines, "</skill>")
	}

	lines = append(lines, "")
	lines = append(lines, "</skills_system>")

	return strings.Join(lines, "\n")
}

// GetTool returns a toolkit.Tool implementation providing the skill access methods.
func (s *Skills) GetTool() toolkit.Tool {
	return newSkillsTool(s)
}

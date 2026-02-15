package skill

import (
	"testing"
)

// TestTwoStageProcess verifies the two-stage loading and activation process
func TestTwoStageProcess(t *testing.T) {
	// Create a mock Skills with multiple loaded skills
	skills := &Skills{
		skills: map[string]Skill{
			"github":    {Name: "github", Description: "GitHub operations"},
			"slack":     {Name: "slack", Description: "Slack messaging"},
			"weather":   {Name: "weather", Description: "Weather info"},
			"discord":   {Name: "discord", Description: "Discord bot"},
			"notion":    {Name: "notion", Description: "Notion workspace"},
			"trello":    {Name: "trello", Description: "Trello boards"},
			"summarize": {Name: "summarize", Description: "Content summarization"},
		},
	}

	// STAGE 1: All skills are loaded
	if len(skills.skills) != 7 {
		t.Errorf("Expected 7 skills to be loaded, got %d", len(skills.skills))
	}

	// STAGE 2 - Scenario A: No SkillsToUse specified - all skills active
	t.Run("All skills active by default", func(t *testing.T) {
		// Don't set activeSkills (simulates not specifying SkillsToUse)
		skills.activeSkills = nil

		// All loaded skills should be active
		for name := range skills.skills {
			if !skills.isSkillActive(name) {
				t.Errorf("Skill %s should be active when no SkillsToUse is specified", name)
			}
		}

		// System prompt should include all skills
		snippet := skills.GetSystemPromptSnippet()
		for name := range skills.skills {
			if !contains(snippet, name) {
				t.Errorf("System prompt should contain skill %s", name)
			}
		}
	})

	// STAGE 2 - Scenario B: Specific SkillsToUse - only those active
	t.Run("Only specified skills active", func(t *testing.T) {
		// Simulate SkillsToUse: []string{"github", "weather"}
		skills.SetActiveSkills([]string{"github", "weather"})

		// Verify stage 1: All skills are still loaded
		if len(skills.skills) != 7 {
			t.Errorf("All 7 skills should still be loaded, got %d", len(skills.skills))
		}

		// Verify stage 2: Only github and weather are active
		activeSkills := map[string]bool{
			"github":  true,
			"weather": true,
		}

		for name := range skills.skills {
			shouldBeActive := activeSkills[name]
			isActive := skills.isSkillActive(name)

			if shouldBeActive && !isActive {
				t.Errorf("Skill %s should be active", name)
			}
			if !shouldBeActive && isActive {
				t.Errorf("Skill %s should NOT be active", name)
			}
		}

		// System prompt should only include active skills
		snippet := skills.GetSystemPromptSnippet()
		if !contains(snippet, "github") {
			t.Error("System prompt should contain github")
		}
		if !contains(snippet, "weather") {
			t.Error("System prompt should contain weather")
		}
		if contains(snippet, "slack") {
			t.Error("System prompt should NOT contain slack (inactive)")
		}
		if contains(snippet, "discord") {
			t.Error("System prompt should NOT contain discord (inactive)")
		}
	})

	// STAGE 2 - Scenario C: Empty SkillsToUse array - all active
	t.Run("Empty SkillsToUse means all active", func(t *testing.T) {
		// Simulate SkillsToUse: []string{} (empty array)
		skills.SetActiveSkills([]string{})

		// All skills should be active
		for name := range skills.skills {
			if !skills.isSkillActive(name) {
				t.Errorf("Skill %s should be active with empty SkillsToUse", name)
			}
		}
	})
}

// TestGetActiveSkills verifies the GetActiveSkills method
func TestGetActiveSkills(t *testing.T) {
	skills := &Skills{
		skills: map[string]Skill{
			"skill1": {Name: "skill1"},
			"skill2": {Name: "skill2"},
			"skill3": {Name: "skill3"},
		},
	}

	t.Run("Returns all skills when none are set as active", func(t *testing.T) {
		active := skills.GetActiveSkills()
		if len(active) != 3 {
			t.Errorf("Expected 3 active skills, got %d", len(active))
		}
	})

	t.Run("Returns only active skills when set", func(t *testing.T) {
		skills.SetActiveSkills([]string{"skill1"})
		active := skills.GetActiveSkills()
		if len(active) != 1 {
			t.Errorf("Expected 1 active skill, got %d", len(active))
		}
		if active[0] != "skill1" {
			t.Errorf("Expected active skill to be 'skill1', got '%s'", active[0])
		}
	})
}

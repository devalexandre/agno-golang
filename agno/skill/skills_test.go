package skill

import (
	"testing"
)

func TestSetActiveSkills(t *testing.T) {
	// Create a mock Skills with some loaded skills
	skills := &Skills{
		skills: map[string]Skill{
			"skill1": {Name: "skill1", Description: "Test skill 1"},
			"skill2": {Name: "skill2", Description: "Test skill 2"},
			"skill3": {Name: "skill3", Description: "Test skill 3"},
		},
	}

	// Test 1: No active skills set - all should be active
	if !skills.isSkillActive("skill1") {
		t.Error("skill1 should be active when no active skills are set")
	}
	if !skills.isSkillActive("skill2") {
		t.Error("skill2 should be active when no active skills are set")
	}

	// Test 2: Set specific active skills
	skills.SetActiveSkills([]string{"skill1", "skill3"})

	if !skills.isSkillActive("skill1") {
		t.Error("skill1 should be active")
	}
	if skills.isSkillActive("skill2") {
		t.Error("skill2 should NOT be active")
	}
	if !skills.isSkillActive("skill3") {
		t.Error("skill3 should be active")
	}

	// Test 3: GetActiveSkills returns correct list
	active := skills.GetActiveSkills()
	if len(active) != 2 {
		t.Errorf("Expected 2 active skills, got %d", len(active))
	}

	// Test 4: GetSystemPromptSnippet only includes active skills
	snippet := skills.GetSystemPromptSnippet()
	if snippet == "" {
		t.Error("System prompt snippet should not be empty")
	}

	// Should contain skill1 and skill3, but not skill2
	if !contains(snippet, "skill1") {
		t.Error("Snippet should contain skill1")
	}
	if !contains(snippet, "skill3") {
		t.Error("Snippet should contain skill3")
	}
	if contains(snippet, "skill2") {
		t.Error("Snippet should NOT contain skill2")
	}
}

func TestIsSkillActiveWithEmptyList(t *testing.T) {
	skills := &Skills{
		skills: map[string]Skill{
			"skill1": {Name: "skill1"},
		},
		activeSkills: []string{}, // Empty list
	}

	// With empty active skills list, all skills should be active
	if !skills.isSkillActive("skill1") {
		t.Error("skill1 should be active with empty active skills list")
	}
}

func TestIsSkillActiveWithNilList(t *testing.T) {
	skills := &Skills{
		skills: map[string]Skill{
			"skill1": {Name: "skill1"},
		},
		activeSkills: nil, // Nil list
	}

	// With nil active skills list, all skills should be active
	if !skills.isSkillActive("skill1") {
		t.Error("skill1 should be active with nil active skills list")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

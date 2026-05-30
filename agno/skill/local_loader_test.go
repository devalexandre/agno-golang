package skill

import (
	"path/filepath"
	"testing"
)

func TestLocalSkillsMissingPathWithValidationDisabled(t *testing.T) {
	loader := NewLocalSkills(filepath.Join(t.TempDir(), "missing"), WithValidation(false))

	skills, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() returned error with validation disabled: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("Load() returned %d skills for a missing path, want 0", len(skills))
	}
}

func TestLocalSkillsMissingPathWithValidationEnabled(t *testing.T) {
	loader := NewLocalSkills(filepath.Join(t.TempDir(), "missing"))

	if _, err := loader.Load(); err == nil {
		t.Fatal("Load() returned nil error with validation enabled for a missing path")
	}
}

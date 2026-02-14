package skill

// SkillLoader is an interface for loading skills from various sources
// (local filesystem, GitHub, URLs, etc.).
type SkillLoader interface {
	// Load loads skills from the source.
	Load() ([]Skill, error)
}

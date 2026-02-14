package skill

// Skill represents a skill that an agent can use.
//
// A skill provides structured instructions, reference documentation,
// and optional scripts that an agent can access to perform specific tasks.
type Skill struct {
	// Name is the unique skill name (from folder name or SKILL.md frontmatter).
	Name string
	// Description is a short description of what the skill does.
	Description string
	// Instructions is the full SKILL.md body (the instructions/guidance for the agent).
	Instructions string
	// SourcePath is the filesystem path to the skill folder.
	SourcePath string
	// Scripts is a list of script filenames in the scripts/ subdirectory.
	Scripts []string
	// References is a list of reference filenames in the references/ subdirectory.
	References []string
	// Metadata is optional metadata from frontmatter (version, author, tags, etc.).
	Metadata map[string]interface{}
	// License is optional license information.
	License string
	// Compatibility is optional compatibility requirements.
	Compatibility string
	// AllowedTools is an optional list of tools this skill is allowed to use.
	AllowedTools []string
}

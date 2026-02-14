package skill

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LocalSkills loads skills from the local filesystem.
//
// It can handle both:
//  1. A single skill folder (contains SKILL.md)
//  2. A directory containing multiple skill folders
type LocalSkills struct {
	path     string
	validate bool
}

// LocalSkillsOption is a functional option for LocalSkills.
type LocalSkillsOption func(*LocalSkills)

// WithValidation controls whether validation is enabled (default: true).
func WithValidation(validate bool) LocalSkillsOption {
	return func(ls *LocalSkills) {
		ls.validate = validate
	}
}

// NewLocalSkills creates a new LocalSkills loader.
func NewLocalSkills(path string, opts ...LocalSkillsOption) *LocalSkills {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	ls := &LocalSkills{
		path:     absPath,
		validate: true,
	}
	for _, opt := range opts {
		opt(ls)
	}
	return ls
}

// Load implements SkillLoader.
func (ls *LocalSkills) Load() ([]Skill, error) {
	if _, err := os.Stat(ls.path); os.IsNotExist(err) {
		return nil, &SkillError{Message: "Skills path does not exist: " + ls.path}
	}

	var skills []Skill

	// Check if this is a single skill folder or a directory of skills
	skillMDPath := filepath.Join(ls.path, "SKILL.md")
	if _, err := os.Stat(skillMDPath); err == nil {
		// Single skill folder
		s, err := ls.loadSkillFromFolder(ls.path)
		if err != nil {
			return nil, err
		}
		if s != nil {
			skills = append(skills, *s)
		}
	} else {
		// Directory of skill folders
		entries, err := os.ReadDir(ls.path)
		if err != nil {
			return nil, &SkillError{Message: "Failed to read skills directory", Cause: err}
		}

		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			itemPath := filepath.Join(ls.path, entry.Name())
			skillMD := filepath.Join(itemPath, "SKILL.md")
			if _, err := os.Stat(skillMD); os.IsNotExist(err) {
				log.Printf("Skipping directory without SKILL.md: %s", itemPath)
				continue
			}

			s, err := ls.loadSkillFromFolder(itemPath)
			if err != nil {
				return nil, err
			}
			if s != nil {
				skills = append(skills, *s)
			}
		}
	}

	log.Printf("Loaded %d skills from %s", len(skills), ls.path)
	return skills, nil
}

// loadSkillFromFolder loads a single skill from a directory.
func (ls *LocalSkills) loadSkillFromFolder(folder string) (*Skill, error) {
	// Validate skill directory structure if validation is enabled
	if ls.validate {
		errors := ValidateSkillDirectory(folder)
		if len(errors) > 0 {
			return nil, &SkillValidationError{
				SkillError: SkillError{Message: "Skill validation failed for '" + filepath.Base(folder) + "'"},
				Errors:     errors,
			}
		}
	}

	skillMDPath := filepath.Join(folder, "SKILL.md")
	content, err := os.ReadFile(skillMDPath)
	if err != nil {
		log.Printf("Error loading skill from %s: %v", folder, err)
		return nil, nil
	}

	frontmatter, instructions, err := parseSkillMD(string(content))
	if err != nil {
		log.Printf("Error parsing SKILL.md from %s: %v", folder, err)
		return nil, nil
	}

	// Get skill name from frontmatter or folder name
	name := filepath.Base(folder)
	if n, ok := frontmatter["name"].(string); ok && n != "" {
		name = n
	}

	description := ""
	if d, ok := frontmatter["description"].(string); ok {
		description = d
	}

	license := ""
	if l, ok := frontmatter["license"].(string); ok {
		license = l
	}

	compatibility := ""
	if c, ok := frontmatter["compatibility"].(string); ok {
		compatibility = c
	}

	var metadata map[string]interface{}
	if m, ok := frontmatter["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	var allowedTools []string
	if at, ok := frontmatter["allowed-tools"].([]interface{}); ok {
		for _, t := range at {
			if s, ok := t.(string); ok {
				allowedTools = append(allowedTools, s)
			}
		}
	}

	scripts := discoverScripts(folder)
	references := discoverReferences(folder)

	return &Skill{
		Name:          name,
		Description:   description,
		Instructions:  instructions,
		SourcePath:    folder,
		Scripts:       scripts,
		References:    references,
		Metadata:      metadata,
		License:       license,
		Compatibility: compatibility,
		AllowedTools:  allowedTools,
	}, nil
}

// parseSkillMD parses SKILL.md content into frontmatter and instructions body.
func parseSkillMD(content string) (map[string]interface{}, string, error) {
	frontmatter := make(map[string]interface{})
	instructions := content

	// Check for YAML frontmatter (between --- delimiters)
	if !strings.HasPrefix(content, "---") {
		return frontmatter, instructions, nil
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return frontmatter, instructions, nil
	}

	frontmatterStr := parts[1]
	instructions = strings.TrimSpace(parts[2])

	if err := yaml.Unmarshal([]byte(frontmatterStr), &frontmatter); err != nil {
		log.Printf("Error parsing YAML frontmatter: %v", err)
		// Fallback: simple key-value parsing
		frontmatter = parseSimpleFrontmatter(frontmatterStr)
	}

	return frontmatter, instructions, nil
}

// parseSimpleFrontmatter is a fallback parser for basic key: value pairs.
func parseSimpleFrontmatter(text string) map[string]interface{} {
	result := make(map[string]interface{})
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			value = strings.Trim(value, "\"'")
			result[key] = value
		}
	}
	return result
}

// discoverScripts finds script files in the scripts/ subdirectory.
func discoverScripts(folder string) []string {
	scriptsDir := filepath.Join(folder, "scripts")
	info, err := os.Stat(scriptsDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return nil
	}

	var scripts []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			scripts = append(scripts, entry.Name())
		}
	}
	sort.Strings(scripts)
	return scripts
}

// discoverReferences finds reference files in the references/ subdirectory.
func discoverReferences(folder string) []string {
	refsDir := filepath.Join(folder, "references")
	info, err := os.Stat(refsDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(refsDir)
	if err != nil {
		return nil
	}

	var references []string
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			references = append(references, entry.Name())
		}
	}
	sort.Strings(references)
	return references
}

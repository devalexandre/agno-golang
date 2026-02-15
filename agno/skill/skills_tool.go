package skill

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// SkillsTool implements toolkit.Tool for accessing skills.
type SkillsTool struct {
	toolkit.Toolkit
	skills *Skills
}

// GetInstructionsParams is the input for GetInstructions.
type GetInstructionsParams struct {
	SkillName string `json:"skill_name" description:"The name of the skill to get instructions for" required:"true"`
}

// GetReferenceParams is the input for GetReference.
type GetReferenceParams struct {
	SkillName     string `json:"skill_name" description:"The name of the skill" required:"true"`
	ReferencePath string `json:"reference_path" description:"The filename of the reference document" required:"true"`
}

// GetScriptParams is the input for GetScript.
type GetScriptParams struct {
	SkillName  string   `json:"skill_name" description:"The name of the skill" required:"true"`
	ScriptPath string   `json:"script_path" description:"The filename of the script" required:"true"`
	Execute    bool     `json:"execute" description:"If true, execute the script. If false (default), return content"`
	Args       []string `json:"args" description:"Optional arguments to pass to the script (only used if execute=true)"`
	Timeout    int      `json:"timeout" description:"Maximum execution time in seconds (default: 30, only used if execute=true)"`
}

// newSkillsTool creates a new SkillsTool for the given Skills orchestrator.
func newSkillsTool(skills *Skills) *SkillsTool {
	tk := toolkit.NewToolkit()
	tk.Name = "Skills"
	tk.Description = "Access agent skills including instructions, reference documents, and scripts"

	st := &SkillsTool{
		Toolkit: tk,
		skills:  skills,
	}

	st.Toolkit.Register(
		"GetInstructions",
		"Load the full instructions for a skill. Use this when you need to follow a skill's guidance.",
		st, st.GetInstructions, GetInstructionsParams{},
	)
	st.Toolkit.Register(
		"GetReference",
		"Load a reference document from a skill's references. Use this to access detailed documentation.",
		st, st.GetReference, GetReferenceParams{},
	)
	st.Toolkit.Register(
		"GetScript",
		"Read or execute a script from a skill. Set execute=true to run the script and get output, or execute=false (default) to read the script content.",
		st, st.GetScript, GetScriptParams{},
	)

	return st
}

// GetInstructions loads the full instructions for a skill.
func (st *SkillsTool) GetInstructions(params GetInstructionsParams) (interface{}, error) {
	// Check if skill is active
	if !st.skills.isSkillActive(params.SkillName) {
		available := strings.Join(st.skills.GetActiveSkills(), ", ")
		return marshalJSON(map[string]interface{}{
			"error":            fmt.Sprintf("Skill '%s' is not active or not found", params.SkillName),
			"available_skills": available,
		}), nil
	}

	s, ok := st.skills.GetSkill(params.SkillName)
	if !ok {
		available := strings.Join(st.skills.GetActiveSkills(), ", ")
		return marshalJSON(map[string]interface{}{
			"error":            fmt.Sprintf("Skill '%s' not found", params.SkillName),
			"available_skills": available,
		}), nil
	}

	return marshalJSON(map[string]interface{}{
		"skill_name":           s.Name,
		"description":          s.Description,
		"instructions":         s.Instructions,
		"available_scripts":    s.Scripts,
		"available_references": s.References,
	}), nil
}

// GetReference loads a reference document from a skill.
func (st *SkillsTool) GetReference(params GetReferenceParams) (interface{}, error) {
	// Check if skill is active
	if !st.skills.isSkillActive(params.SkillName) {
		available := strings.Join(st.skills.GetActiveSkills(), ", ")
		return marshalJSON(map[string]interface{}{
			"error":            fmt.Sprintf("Skill '%s' is not active or not found", params.SkillName),
			"available_skills": available,
		}), nil
	}

	s, ok := st.skills.GetSkill(params.SkillName)
	if !ok {
		available := strings.Join(st.skills.GetActiveSkills(), ", ")
		return marshalJSON(map[string]interface{}{
			"error":            fmt.Sprintf("Skill '%s' not found", params.SkillName),
			"available_skills": available,
		}), nil
	}

	if !containsString(s.References, params.ReferencePath) {
		return marshalJSON(map[string]interface{}{
			"error":                fmt.Sprintf("Reference '%s' not found in skill '%s'", params.ReferencePath, params.SkillName),
			"available_references": s.References,
		}), nil
	}

	// Validate path to prevent path traversal attacks
	refsDir := filepath.Join(s.SourcePath, "references")
	if !IsSafePath(refsDir, params.ReferencePath) {
		return marshalJSON(map[string]interface{}{
			"error":      fmt.Sprintf("Invalid reference path: '%s'", params.ReferencePath),
			"skill_name": params.SkillName,
		}), nil
	}

	refFile := filepath.Join(refsDir, params.ReferencePath)
	content, err := ReadFileSafe(refFile)
	if err != nil {
		return marshalJSON(map[string]interface{}{
			"error":          fmt.Sprintf("Error reading reference file: %v", err),
			"skill_name":     params.SkillName,
			"reference_path": params.ReferencePath,
		}), nil
	}

	return marshalJSON(map[string]interface{}{
		"skill_name":     params.SkillName,
		"reference_path": params.ReferencePath,
		"content":        content,
	}), nil
}

// GetScript reads or executes a script from a skill.
func (st *SkillsTool) GetScript(params GetScriptParams) (interface{}, error) {
	// Check if skill is active
	if !st.skills.isSkillActive(params.SkillName) {
		available := strings.Join(st.skills.GetActiveSkills(), ", ")
		return marshalJSON(map[string]interface{}{
			"error":            fmt.Sprintf("Skill '%s' is not active or not found", params.SkillName),
			"available_skills": available,
		}), nil
	}

	s, ok := st.skills.GetSkill(params.SkillName)
	if !ok {
		available := strings.Join(st.skills.GetActiveSkills(), ", ")
		return marshalJSON(map[string]interface{}{
			"error":            fmt.Sprintf("Skill '%s' not found", params.SkillName),
			"available_skills": available,
		}), nil
	}

	if !containsString(s.Scripts, params.ScriptPath) {
		return marshalJSON(map[string]interface{}{
			"error":             fmt.Sprintf("Script '%s' not found in skill '%s'", params.ScriptPath, params.SkillName),
			"available_scripts": s.Scripts,
		}), nil
	}

	// Validate path to prevent path traversal attacks
	scriptsDir := filepath.Join(s.SourcePath, "scripts")
	if !IsSafePath(scriptsDir, params.ScriptPath) {
		return marshalJSON(map[string]interface{}{
			"error":      fmt.Sprintf("Invalid script path: '%s'", params.ScriptPath),
			"skill_name": params.SkillName,
		}), nil
	}

	scriptFile := filepath.Join(scriptsDir, params.ScriptPath)

	if !params.Execute {
		// Read mode: return script content
		content, err := ReadFileSafe(scriptFile)
		if err != nil {
			return marshalJSON(map[string]interface{}{
				"error":       fmt.Sprintf("Error reading script file: %v", err),
				"skill_name":  params.SkillName,
				"script_path": params.ScriptPath,
			}), nil
		}
		return marshalJSON(map[string]interface{}{
			"skill_name":  params.SkillName,
			"script_path": params.ScriptPath,
			"content":     content,
		}), nil
	}

	// Execute mode: run the script
	timeout := time.Duration(params.Timeout) * time.Second
	if params.Timeout <= 0 {
		timeout = 30 * time.Second
	}

	result, err := RunScript(scriptFile, params.Args, timeout, s.SourcePath)
	if err != nil {
		if strings.Contains(err.Error(), "timed out") {
			return marshalJSON(map[string]interface{}{
				"error":       fmt.Sprintf("Script execution timed out after %d seconds", params.Timeout),
				"skill_name":  params.SkillName,
				"script_path": params.ScriptPath,
			}), nil
		}
		return marshalJSON(map[string]interface{}{
			"error":       fmt.Sprintf("Error executing script: %v", err),
			"skill_name":  params.SkillName,
			"script_path": params.ScriptPath,
		}), nil
	}

	return marshalJSON(map[string]interface{}{
		"skill_name":  params.SkillName,
		"script_path": params.ScriptPath,
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"returncode":  result.ReturnCode,
	}), nil
}

// marshalJSON marshals a value to a JSON string, returning empty object on error.
func marshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// containsString checks if a slice contains a specific string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

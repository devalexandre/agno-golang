package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	MaxSkillNameLength     = 64
	MaxDescriptionLength   = 1024
	MaxCompatibilityLength = 500
)

var allowedFields = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"allowed-tools": true,
	"metadata":      true,
	"compatibility": true,
}

func validateName(name string, skillDirName string) []string {
	var errors []string

	name = strings.TrimSpace(name)
	if name == "" {
		return []string{"Field 'name' must be a non-empty string"}
	}

	if len(name) > MaxSkillNameLength {
		errors = append(errors, fmt.Sprintf("Skill name '%s' exceeds %d character limit (%d chars)", name, MaxSkillNameLength, len(name)))
	}

	if name != strings.ToLower(name) {
		errors = append(errors, fmt.Sprintf("Skill name '%s' must be lowercase", name))
	}

	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		errors = append(errors, "Skill name cannot start or end with a hyphen")
	}

	if strings.Contains(name, "--") {
		errors = append(errors, "Skill name cannot contain consecutive hyphens")
	}

	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '-' {
			errors = append(errors, fmt.Sprintf("Skill name '%s' contains invalid characters. Only letters, digits, and hyphens are allowed.", name))
			break
		}
	}

	if skillDirName != "" && skillDirName != name {
		errors = append(errors, fmt.Sprintf("Directory name '%s' must match skill name '%s'", skillDirName, name))
	}

	return errors
}

func validateDescription(description string) []string {
	var errors []string

	description = strings.TrimSpace(description)
	if description == "" {
		return []string{"Field 'description' must be a non-empty string"}
	}

	if len(description) > MaxDescriptionLength {
		errors = append(errors, fmt.Sprintf("Description exceeds %d character limit (%d chars)", MaxDescriptionLength, len(description)))
	}

	return errors
}

func validateCompatibility(compatibility string) []string {
	var errors []string
	if len(compatibility) > MaxCompatibilityLength {
		errors = append(errors, fmt.Sprintf("Compatibility exceeds %d character limit (%d chars)", MaxCompatibilityLength, len(compatibility)))
	}
	return errors
}

func validateLicense(license string) []string {
	// License is a free-form string, just needs to be non-empty if present
	return nil
}

func validateAllowedTools(allowedTools interface{}) []string {
	var errors []string
	list, ok := allowedTools.([]interface{})
	if !ok {
		return []string{"Field 'allowed-tools' must be a list"}
	}
	for _, item := range list {
		if _, ok := item.(string); !ok {
			errors = append(errors, "Field 'allowed-tools' must be a list of strings")
			break
		}
	}
	return errors
}

func validateMetadataValue(metadata interface{}) []string {
	if _, ok := metadata.(map[string]interface{}); !ok {
		return []string{"Field 'metadata' must be a dictionary"}
	}
	return nil
}

func validateMetadataFields(metadata map[string]interface{}) []string {
	var errors []string
	var extra []string
	for key := range metadata {
		if !allowedFields[key] {
			extra = append(extra, key)
		}
	}
	if len(extra) > 0 {
		allowed := make([]string, 0, len(allowedFields))
		for k := range allowedFields {
			allowed = append(allowed, k)
		}
		errors = append(errors, fmt.Sprintf("Unexpected fields in frontmatter: %s. Only %v are allowed.", strings.Join(extra, ", "), allowed))
	}
	return errors
}

// ValidateMetadata validates parsed skill metadata.
func ValidateMetadata(metadata map[string]interface{}, skillDirName string) []string {
	var errors []string

	errors = append(errors, validateMetadataFields(metadata)...)

	if name, ok := metadata["name"]; ok {
		if nameStr, ok := name.(string); ok {
			errors = append(errors, validateName(nameStr, skillDirName)...)
		} else {
			errors = append(errors, "Field 'name' must be a string")
		}
	} else {
		errors = append(errors, "Missing required field in frontmatter: name")
	}

	if desc, ok := metadata["description"]; ok {
		if descStr, ok := desc.(string); ok {
			errors = append(errors, validateDescription(descStr)...)
		} else {
			errors = append(errors, "Field 'description' must be a string")
		}
	} else {
		errors = append(errors, "Missing required field in frontmatter: description")
	}

	if compat, ok := metadata["compatibility"]; ok {
		if compatStr, ok := compat.(string); ok {
			errors = append(errors, validateCompatibility(compatStr)...)
		}
	}

	if license, ok := metadata["license"]; ok {
		if licenseStr, ok := license.(string); ok {
			errors = append(errors, validateLicense(licenseStr)...)
		}
	}

	if at, ok := metadata["allowed-tools"]; ok {
		errors = append(errors, validateAllowedTools(at)...)
	}

	if md, ok := metadata["metadata"]; ok {
		errors = append(errors, validateMetadataValue(md)...)
	}

	return errors
}

// ValidateSkillDirectory validates a skill directory structure and contents.
func ValidateSkillDirectory(skillDir string) []string {
	info, err := os.Stat(skillDir)
	if err != nil {
		return []string{fmt.Sprintf("Path does not exist: %s", skillDir)}
	}
	if !info.IsDir() {
		return []string{fmt.Sprintf("Not a directory: %s", skillDir)}
	}

	skillMD := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMD); os.IsNotExist(err) {
		return []string{"Missing required file: SKILL.md"}
	}

	content, err := os.ReadFile(skillMD)
	if err != nil {
		return []string{fmt.Sprintf("Error reading SKILL.md: %v", err)}
	}

	contentStr := string(content)
	if !strings.HasPrefix(contentStr, "---") {
		return []string{"SKILL.md must start with YAML frontmatter (---)"}
	}

	parts := strings.SplitN(contentStr, "---", 3)
	if len(parts) < 3 {
		return []string{"SKILL.md frontmatter not properly closed with ---"}
	}

	frontmatterStr := parts[1]
	var metadata map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatterStr), &metadata); err != nil {
		return []string{fmt.Sprintf("Invalid YAML in frontmatter: %v", err)}
	}

	if metadata == nil {
		return []string{"SKILL.md frontmatter must be a YAML mapping"}
	}

	dirName := filepath.Base(skillDir)
	return ValidateMetadata(metadata, dirName)
}

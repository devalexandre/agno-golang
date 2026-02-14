# Skills

Skills are reusable packages of domain expertise that extend an agent's capabilities. Each skill is a self-contained directory with instructions, optional scripts, and reference documents that the agent can access on demand.

Unlike tools (which give the agent new *actions*), skills give the agent new *knowledge and workflows*. A tool lets the agent call an API; a skill teaches the agent *when* and *how* to call it.

## How It Works

The skills system integrates with the agent in two ways:

1. **System prompt**: A compact XML snippet listing all available skills (name + description) is injected into the agent's system message. This lets the agent know what skills exist without loading full instructions.
2. **Tools**: Three tool functions are registered so the agent can load skill content on demand:
   - `Skills_GetInstructions` - Load full instructions for a skill
   - `Skills_GetReference` - Load a reference document
   - `Skills_GetScript` - Read or execute a script

This progressive disclosure approach keeps the context window small while giving the agent access to deep domain knowledge when needed.

## Skill Structure

```
my-skill/
  SKILL.md              # Required: YAML frontmatter + markdown instructions
  scripts/              # Optional: executable scripts
    check_style.sh
  references/           # Optional: reference documentation
    style-guide.md
```

### SKILL.md

Every skill must have a `SKILL.md` file with YAML frontmatter and a markdown body:

```markdown
---
name: code-review
description: Code review assistance with linting, style checking, and best practices
license: Apache-2.0
metadata:
  version: "1.0.0"
  author: your-name
  tags: ["quality", "review"]
---

# Code Review Skill

Instructions for the agent go here...
```

**Required fields**: `name`, `description`

**Optional fields**: `license`, `metadata`, `compatibility`, `allowed-tools`

### Naming Rules

- Lowercase letters, digits, and hyphens only
- Under 64 characters
- Directory name must match the `name` field in frontmatter
- Prefer short, verb-led phrases: `code-review`, `git-workflow`

## Usage

### 1. Create a skill directory

```bash
mkdir -p my-skills/code-review/{scripts,references}
```

Write the `SKILL.md` with frontmatter and instructions. Add scripts and references as needed.

### 2. Load skills into the agent

```go
package main

import (
    "context"
    "log"

    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models/ollama"
    "github.com/devalexandre/agno-golang/agno/models"
    "github.com/devalexandre/agno-golang/agno/skill"
)

func main() {
    ctx := context.Background()

    // Create a loader pointing to a directory of skills
    loader := skill.NewLocalSkills("./my-skills")

    // Create the Skills orchestrator
    skills, err := skill.NewSkills(loader)
    if err != nil {
        log.Fatal(err)
    }

    // Create model
    model, _ := ollama.NewOllamaChat(
        models.WithID("llama3.2:latest"),
    )

    // Create agent with skills
    a, _ := agent.NewAgent(agent.AgentConfig{
        Context:      ctx,
        Model:        model,
        Name:         "My Agent",
        Instructions: "You are a helpful assistant with specialized skills.",
        Skills:       skills,
    })

    resp, _ := a.Run("Review this Go code for issues: func foo() { fmt.Println(\"hello\") }")
    fmt.Println(resp.TextContent)
}
```

### 3. Multiple loaders

You can combine skills from different sources:

```go
projectSkills := skill.NewLocalSkills("./project-skills")
sharedSkills := skill.NewLocalSkills("/opt/shared-skills")

skills, err := skill.NewSkills(projectSkills, sharedSkills)
```

### 4. Single skill loading

Point directly to a single skill folder (one that contains `SKILL.md`):

```go
loader := skill.NewLocalSkills("./my-skills/code-review")
```

The loader auto-detects whether the path is a single skill or a directory of skills.

### 5. Disable validation

Validation is on by default. To skip it:

```go
loader := skill.NewLocalSkills("./my-skills", skill.WithValidation(false))
```

## What Happens at Runtime

When the agent starts:

1. The loader reads each skill directory, parses `SKILL.md` frontmatter, and discovers `scripts/` and `references/` contents.
2. The system prompt receives a `<skills_system>` XML block listing every skill's name and description.
3. Three tool functions are registered: `Skills_GetInstructions`, `Skills_GetReference`, `Skills_GetScript`.

When the agent receives a user message:

1. The agent sees the skill summaries in its system prompt.
2. If the task matches a skill, the agent calls `Skills_GetInstructions("code-review")` to load full instructions.
3. Based on the instructions, it may call `Skills_GetReference` or `Skills_GetScript` to access specific resources.

## Tool Reference

### Skills_GetInstructions

Load the full instructions for a skill.

```json
{
  "skill_name": "code-review"
}
```

Returns the skill's description, full markdown instructions, and lists of available scripts and references.

### Skills_GetReference

Load a reference document from a skill.

```json
{
  "skill_name": "code-review",
  "reference_path": "style-guide.md"
}
```

Returns the raw content of the reference file.

### Skills_GetScript

Read or execute a script from a skill.

```json
{
  "skill_name": "code-review",
  "script_path": "check_style.sh",
  "execute": false
}
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `skill_name` | string | required | Skill name |
| `script_path` | string | required | Script filename |
| `execute` | bool | `false` | If true, execute the script and return stdout/stderr |
| `args` | []string | `[]` | Arguments passed to the script (only when `execute=true`) |
| `timeout` | int | `30` | Max execution time in seconds (only when `execute=true`) |

## Writing Good Skills

### Be concise

The agent is already smart. Only add context it doesn't have. Challenge each line: "Does the agent really need this?"

### Progressive disclosure

- **Frontmatter** (name + description): Always in the system prompt (~100 words max)
- **SKILL.md body**: Loaded on demand when the skill triggers
- **References/Scripts**: Loaded only when specifically needed

Keep SKILL.md under 500 lines. Split long content into reference files.

### Match freedom to task

- **High freedom** (text instructions): When multiple approaches are valid
- **Medium freedom** (scripts with parameters): When a preferred pattern exists
- **Low freedom** (specific scripts): When operations are fragile or must be exact

### What NOT to include

- README.md, CHANGELOG.md, or other auxiliary docs
- Installation guides or setup procedures for the skill itself
- User-facing documentation (skills are for the agent, not the user)

## Built-in Skills

The project includes ready-to-use skills in the `skills/` directory:

| Skill | Description |
|-------|-------------|
| `github` | GitHub operations via `gh` CLI (PRs, issues, releases) |
| `slack` | Slack Web API (messages, reactions, channels) |
| `discord` | Discord REST API (messages, threads, reactions) |
| `notion` | Notion API (pages, databases, blocks) |
| `trello` | Trello REST API (boards, lists, cards) |
| `weather` | Weather data via wttr.in and Open-Meteo (no API key) |
| `summarize` | Summarize URLs, PDFs, and YouTube videos |
| `obsidian` | Obsidian vault management via obsidian-cli |
| `coding-agent` | Orchestrate external coding agents (Codex, Claude Code) |
| `skill-creator` | Guide for creating new skills |

To use them:

```go
loader := skill.NewLocalSkills("./skills")
skills, _ := skill.NewSkills(loader)
```

## Security

- **Path traversal protection**: All reference and script paths are validated against directory escaping (`../`) before access.
- **Script execution**: Scripts run with `context.WithTimeout` and inherit the skill directory as working directory. Output is captured and returned as JSON.
- **Validation**: Enabled by default. Checks SKILL.md structure, name format, description length, and directory layout.

## Architecture

```
agno/skill/
  skill.go          # Skill struct
  skills.go         # Skills orchestrator (loads, queries, generates prompt)
  skills_tool.go    # toolkit.Tool implementation (3 registered methods)
  loader.go         # SkillLoader interface
  local_loader.go   # LocalSkills - filesystem loader with YAML parsing
  validator.go      # Validation rules
  utils.go          # Path safety, script execution, file reading
  errors.go         # SkillError, SkillParseError, SkillValidationError
```

### SkillLoader interface

```go
type SkillLoader interface {
    Load() ([]Skill, error)
}
```

Implement this interface to load skills from other sources (databases, remote URLs, embedded filesystems).

## Cookbook

See `cookbook/agents/skills/` for a complete working example with sample skills for code review and git workflow.

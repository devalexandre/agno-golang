# Skills Examples

This directory contains examples demonstrating different ways to use the Skills system in agno-golang.

## How Skills Work: Two Stages

```
┌─────────────────────────────────────────────────────────────┐
│ STAGE 1: LOADING (Automatic)                               │
│ ─────────────────────────────────────────────────────────── │
│ ALL built-in skills from ./skills are loaded:              │
│ ✓ github, slack, discord, notion, trello, weather,         │
│   summarize, obsidian, coding-agent, skill-creator         │
│                                                             │
│ + Custom skills (if CustomSkillsLoader is provided)        │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ STAGE 2: ACTIVATION (Configurable via SkillsToUse)         │
│ ─────────────────────────────────────────────────────────── │
│ Option A: SkillsToUse NOT specified                        │
│   → Agent can use ALL loaded skills                        │
│                                                             │
│ Option B: SkillsToUse = ["github", "weather"]              │
│   → Agent can ONLY use github and weather                  │
│   → Other skills are loaded but not accessible             │
└─────────────────────────────────────────────────────────────┘
```

## Examples

### 1. Simple Usage (Recommended)
**Directory**: `simple-usage/`

The simplest way to use built-in skills. Just specify which ones you want!

```bash
cd simple-usage
go run main.go
```

**What it shows**:
- Automatic loading of built-in skills from `./skills`
- Using `SkillsToUse` to activate specific skills
- No need to manage loaders manually

### 2. All Skills
**Directory**: `all-skills/`

Use ALL available built-in skills at once.

```bash
cd all-skills
go run main.go
```

**What it shows**:
- How to make all built-in skills available
- Leave `SkillsToUse` empty to activate everything

### 3. Custom Skills
**Directory**: `custom-skills/`

Combine built-in skills with your own custom skills.

```bash
cd custom-skills
go run main.go
```

**What it shows**:
- Using `SkillsToUse` for built-in skills
- Using `CustomSkillsLoader` for your own skills
- Both skill sets work together seamlessly

### 4. With Sample Skills
**Directory**: `with-sample-skills/`

Complete example using local sample skills for code review.

```bash
cd with-sample-skills
go run main.go
```

**What it shows**:
- Loading skills from a local directory
- Using skills with scripts and references
- Manual skill loading (deprecated approach)

### 5. Filtered Skills (Advanced)
**Directory**: `filtered-skills/`

Advanced example showing loader-level filtering.

```bash
cd filtered-skills
go run main.go
```

**What it shows**:
- Using `WithFilter` to load only specific skills
- Combining filtered built-in with all custom skills
- Loader-level filtering (more complex, not recommended)

## Sample Skills

The `sample_skills/` directory contains example skill implementations:

- **code-review**: Code review skill with style checking
- **git-workflow**: Git workflow automation skill

Use these as templates for creating your own custom skills.

## Quick Start

For most use cases, start with the **simple-usage** example:

```go
a, _ := agent.NewAgent(agent.AgentConfig{
    Context: ctx,
    Model:   model,

    // Just list the skills you want!
    SkillsToUse: []string{"github", "weather", "summarize"},
})
```

## Environment Setup

All examples require:
- A valid Together AI API key in `TOGETHER_API_KEY` environment variable
- Go 1.21 or later

```bash
export TOGETHER_API_KEY="your-key-here"
```

## Built-in Skills

Available built-in skills (in `../../skills/`):
- `github` - GitHub operations
- `slack` - Slack messaging
- `discord` - Discord bot actions
- `notion` - Notion workspace management
- `trello` - Trello board operations
- `weather` - Weather information
- `summarize` - Summarize content from URLs/PDFs
- `obsidian` - Obsidian vault management
- `coding-agent` - External coding agent orchestration
- `skill-creator` - Guide for creating new skills

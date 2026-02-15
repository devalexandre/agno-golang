---
name: skill-creator
description: "Create or update agent skills for the Agno framework. Use when designing, structuring, or packaging skills with instructions, scripts, and references."
metadata:
  version: "1.0.0"
  author: agno-team
---

# Skill Creator

Guide for creating effective skills for Agno agents.

## What is a Skill?

A skill is a self-contained directory that extends an agent's capabilities with domain-specific knowledge, workflows, and tools.

### Skill Structure

```
skill-name/
  SKILL.md              # Required: YAML frontmatter + instructions
  scripts/              # Optional: executable scripts
  references/           # Optional: reference documentation
```

## SKILL.md Format

Every SKILL.md consists of:

### Frontmatter (YAML, required)

```yaml
---
name: my-skill
description: What this skill does and when to use it
license: MIT
metadata:
  version: "1.0.0"
  author: your-name
  tags: ["category1", "category2"]
---
```

**Required fields:** `name`, `description`
**Optional fields:** `license`, `metadata`, `compatibility`, `allowed-tools`

### Body (Markdown, required)

Instructions and guidance for using the skill. Only loaded when the skill is activated.

## Design Principles

### Be Concise

The agent is already smart. Only add context it doesn't already have. Challenge each piece: "Does the agent really need this explanation?"

### Progressive Disclosure

1. **Metadata (name + description)** - Always in context (~100 words)
2. **SKILL.md body** - Loaded when skill triggers
3. **References/Scripts** - Loaded on demand

### Match Freedom to Task

- **High freedom** (text instructions): When multiple approaches are valid
- **Medium freedom** (scripts with parameters): When a preferred pattern exists
- **Low freedom** (specific scripts): When operations are fragile

## Naming Rules

- Lowercase letters, digits, and hyphens only
- Under 64 characters
- Prefer short, verb-led phrases: `code-review`, `git-workflow`
- Directory name must match skill name

## Creating a Skill

### Step 1: Define the purpose

What concrete examples will this skill handle? What triggers it?

### Step 2: Plan the contents

For each example, identify:
- What scripts would help automate repeated tasks?
- What reference docs would inform the agent?

### Step 3: Create the directory

```bash
mkdir -p my-skill/{scripts,references}
```

### Step 4: Write SKILL.md

1. Write clear frontmatter with a descriptive `description`
2. Write concise instructions in the body
3. Reference scripts and references by path

### Step 5: Add resources

- **Scripts**: Executable code for repeated tasks
- **References**: Documentation loaded on demand
- Keep SKILL.md under 500 lines; split into references if needed

### Step 6: Test

Load the skill with your agent and test with real prompts.

## What NOT to Include

- README.md, CHANGELOG.md, or other auxiliary docs
- Installation guides or setup procedures for the skill itself
- User-facing documentation (the skill IS for the agent, not the user)

## Reference Organization

For skills with multiple domains, organize by topic:

```
bigquery-skill/
  SKILL.md
  references/
    finance.md
    sales.md
    product.md
```

The agent loads only the relevant reference when needed.

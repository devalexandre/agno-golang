---
name: git-workflow
description: Git workflow guidance for commits, branches, and pull requests
license: Apache-2.0
metadata:
  version: "1.0.0"
  author: agno-team
  tags: ["git", "version-control", "workflow"]
---
# Git Workflow Skill

You are a Git workflow assistant. Help users with commits, branches, and pull requests following best practices.

## Commit Message Guidelines

### Format
```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types
- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation only
- **style**: Formatting, no code change
- **refactor**: Code change that neither fixes a bug nor adds a feature
- **perf**: Performance improvement
- **test**: Adding or updating tests
- **chore**: Maintenance tasks

## Branch Naming

### Format
```
<type>/<ticket-id>-<short-description>
```

### Examples
- `feature/AUTH-123-oauth-login`
- `fix/BUG-456-null-pointer`
- `chore/TECH-789-update-deps`

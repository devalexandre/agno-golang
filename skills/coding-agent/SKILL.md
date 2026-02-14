---
name: coding-agent
description: "Orchestrate external coding agents (Claude Code, Codex CLI, OpenCode) via background processes for automated code tasks, PR reviews, and parallel issue fixing."
metadata:
  version: "1.0.0"
  requires:
    any-bins: ["claude", "codex", "opencode"]
---

# Coding Agent Skill

Use external coding agents via bash for automated code tasks.

## Quick Start: One-Shot Tasks

```bash
# Quick task with Codex (needs a git repo)
SCRATCH=$(mktemp -d) && cd $SCRATCH && git init && codex exec "Your prompt here"

# Claude Code
claude "Fix the bug in main.go"

# OpenCode
opencode run "Add error handling to the API calls"
```

## Background Mode for Long Tasks

```bash
# Start agent in background
codex exec --full-auto "Build a snake game" &
PID=$!

# Check if done
kill -0 $PID 2>/dev/null && echo "Running..." || echo "Done"

# Wait for completion
wait $PID
```

## Codex CLI

### Flags

| Flag | Effect |
|------|--------|
| `exec "prompt"` | One-shot execution, exits when done |
| `--full-auto` | Sandboxed but auto-approves in workspace |
| `--yolo` | No sandbox, no approvals (fastest) |

### Examples

```bash
# Auto-approve mode
codex exec --full-auto "Add dark mode toggle"

# Full auto, no sandbox
codex --yolo "Refactor the auth module"

# PR review
codex review --base origin/main
```

## Claude Code

```bash
# Simple task
claude "Your task description"

# With specific file context
claude "Fix the tests in auth_test.go"
```

## Parallel Issue Fixing with Git Worktrees

Fix multiple issues simultaneously:

```bash
# 1. Create worktrees for each issue
git worktree add -b fix/issue-78 /tmp/issue-78 main
git worktree add -b fix/issue-99 /tmp/issue-99 main

# 2. Launch agents in each
cd /tmp/issue-78 && codex --yolo "Fix issue #78: description" &
cd /tmp/issue-99 && codex --yolo "Fix issue #99: description" &

# 3. Wait for completion
wait

# 4. Create PRs
cd /tmp/issue-78 && git push -u origin fix/issue-78
gh pr create --repo user/repo --head fix/issue-78 --title "fix: issue 78" --body "..."

# 5. Cleanup
git worktree remove /tmp/issue-78
git worktree remove /tmp/issue-99
```

## Batch PR Reviews

```bash
# Fetch all PR refs
git fetch origin '+refs/pull/*/head:refs/remotes/origin/pr/*'

# Review multiple PRs
for pr in 86 87 88; do
  echo "=== Reviewing PR #$pr ==="
  codex exec "Review PR #$pr. git diff origin/main...origin/pr/$pr"
done
```

## Rules

1. Respect tool choice - if user asks for Codex, use Codex
2. Be patient - don't kill sessions because they're "slow"
3. Use `--full-auto` for building, vanilla for reviewing
4. Parallel execution is OK - run many agents at once for batch work
5. Never start agents in the project's own directory for reviews - use worktrees

---
name: github
description: "Interact with GitHub using the gh CLI. Use gh issue, gh pr, gh run, and gh api for issues, PRs, CI runs, and advanced queries."
metadata:
  version: "1.0.0"
  requires:
    bins: ["gh"]
---

# GitHub Skill

Use the `gh` CLI to interact with GitHub. Always specify `--repo owner/repo` when not in a git directory, or use URLs directly.

## Pull Requests

Check CI status on a PR:

```bash
gh pr checks 55 --repo owner/repo
```

List recent workflow runs:

```bash
gh run list --repo owner/repo --limit 10
```

View a run and see which steps failed:

```bash
gh run view <run-id> --repo owner/repo
```

View logs for failed steps only:

```bash
gh run view <run-id> --repo owner/repo --log-failed
```

## Issues

List open issues:

```bash
gh issue list --repo owner/repo --state open
```

Create an issue:

```bash
gh issue create --repo owner/repo --title "Bug: ..." --body "Description..."
```

Close an issue with comment:

```bash
gh issue close 42 --repo owner/repo --comment "Fixed in PR #55"
```

## API for Advanced Queries

The `gh api` command is useful for accessing data not available through other subcommands.

Get PR with specific fields:

```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state, .user.login'
```

List PR review comments:

```bash
gh api repos/owner/repo/pulls/55/comments --jq '.[].body'
```

## JSON Output

Most commands support `--json` for structured output. You can use `--jq` to filter:

```bash
gh issue list --repo owner/repo --json number,title --jq '.[] | "\(.number): \(.title)"'
```

## Useful Patterns

Create PR with body:

```bash
gh pr create --repo owner/repo --title "feat: add feature" --body "## Summary\n- Added X\n- Fixed Y"
```

Review and merge PR:

```bash
gh pr review 55 --repo owner/repo --approve
gh pr merge 55 --repo owner/repo --squash
```

Check PR diff:

```bash
gh pr diff 55 --repo owner/repo
```

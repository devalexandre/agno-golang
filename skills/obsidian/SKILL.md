---
name: obsidian
description: "Work with Obsidian vaults (plain Markdown notes). Search, create, move, and edit notes."
metadata:
  version: "1.0.0"
  requires:
    bins: ["obsidian-cli"]
---

# Obsidian Skill

Obsidian vaults are normal folders on disk containing Markdown files.

## Vault Structure

- Notes: `*.md` (plain text Markdown)
- Config: `.obsidian/` (workspace + plugin settings)
- Canvases: `*.canvas` (JSON)
- Attachments: images/PDFs in configured folder

## Find the Active Vault

On macOS, vaults are tracked in:
`~/Library/Application Support/obsidian/obsidian.json`

Using obsidian-cli:

```bash
obsidian-cli print-default --path-only
```

## obsidian-cli Quick Start

### Set a default vault

```bash
obsidian-cli set-default "<vault-folder-name>"
obsidian-cli print-default
```

### Search notes

```bash
# Search by note name
obsidian-cli search "query"

# Search inside notes (content search)
obsidian-cli search-content "query"
```

### Create a note

```bash
obsidian-cli create "Folder/New note" --content "# My Note\n\nContent here" --open
```

### Move/rename a note (updates wikilinks)

```bash
obsidian-cli move "old/path/note" "new/path/note"
```

### Delete a note

```bash
obsidian-cli delete "path/note"
```

## Direct File Editing

You can also edit `.md` files directly with any editor. Obsidian will pick up changes automatically.

## Tips

- Multiple vaults are common (work/personal). Don't guess; check the config
- Avoid hardcoding vault paths in scripts; use `print-default` instead
- Wikilinks `[[note]]` are the preferred link format in Obsidian
- Use `obsidian-cli move` instead of `mv` to update cross-references

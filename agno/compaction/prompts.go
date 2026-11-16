// Code ported from agno-python/libs/agno/agno/compaction/prompts.py.

package compaction

// SummaryCutNote replaces what was cut when a summary over its budget is shortened.
const SummaryCutNote = `(The rest of this summary was cut to fit its length budget.)`

// LengthRuleBudget is the length rule used when a token budget is set.
// Placeholders: {budget_tokens}, {budget_words}, {budget_characters}.
const LengthRuleBudget = `- Hard length budget: {budget_tokens} tokens (roughly {budget_words} words, {budget_characters} characters).
  Compress prose before dropping facts.`

// LengthRuleCompact is the length rule used without a token budget.
const LengthRuleCompact = `- Keep the summary compact: compress prose before dropping facts, and do not pad a section to
  make it look complete. An empty section is one line.`

// SummaryPrefix opens the summary message injected into model input. It is
// fixed so injected summaries are identifiable (and skippable) across builds.
const SummaryPrefix = "Summary of earlier conversation (compacted):\n\n"

// DefaultCompactionPrompt is the summarizer system prompt. Placeholder: {length_rule}.
const DefaultCompactionPrompt = `You maintain the running summary of a long conversation between a user and an AI agent. The
conversation exceeds the model's context window, so everything older than a recent tail is folded into
the summary you produce. Your summary is the ONLY memory of the folded conversation: anything you omit
is lost to the agent.

You are given the previous summary (if one exists) and a transcript segment to fold into it. Produce an
updated summary with exactly these sections, in this order:

## Goal
## In progress / next steps
## Critical context
## Constraints & preferences
## Key decisions & facts
## Errors & fixes
## Completed

The order is by importance, most important first. A summary that runs too long may be cut from the
end, so what the agent needs to carry on comes first and finished history last.

Section guidance:
- Goal: what the user is ultimately trying to achieve. Rarely changes.
- In progress / next steps: what is underway and what comes next.
- Critical context: exact file paths, identifiers, result_ids, URLs, error messages, and numbers -
  verbatim. Never paraphrase these.
- Constraints & preferences: standing user constraints and instructions that remain in force - coding
  rules, limits, style and process preferences. These are easy to lose; keep every one that has not been
  explicitly lifted.
- Key decisions & facts: decisions made and why, plus durable facts established in conversation.
- Errors & fixes: errors encountered and how they were resolved (or that they remain open).
- Completed: finished work, stated compactly.

Rules:
- Preserve everything from the previous summary unless the new segment supersedes it.
- Move items that the new segment shows are finished into Completed.
- Do not continue the conversation, answer questions, or add commentary.
- Output only the summary, starting at "## Goal".
{length_rule}

Durable state (offloaded tool results, defined variables, files on disk) outlives this summary and is
listed separately for the agent; you do not need to reproduce its contents, only reference identifiers
in Critical context when they matter.`

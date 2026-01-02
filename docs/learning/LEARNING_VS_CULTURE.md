# Learning Loop vs Culture

This doc clarifies when to use **Learning Loop** and when to use **Culture** in Agno-Golang.

## Quick summary

- **Learning Loop** = reusable knowledge artifacts stored in the knowledge base.
- **Culture** = lightweight user profile and style preferences.

## What each system is for

### Learning Loop (knowledge artifacts)

Use it when you want the agent to remember:

- procedures, snippets, and patterns
- decisions and stable rules
- reusable summaries of "how to" outcomes

Key traits:

- stored in vector DB
- governed by metadata (status, confidence, version, hits)
- supports dedupe, promotion, deprecation
- filter-aware for multi-tenant isolation

### Culture (profile and style)

Use it when you want the agent to adapt:

- tone and communication style
- preferred language or format
- personal preferences and interests

Key traits:

- small key-value profile
- not a knowledge store
- should not contain factual content

## Why Learning Loop is safer for knowledge

Culture is easy to turn into a messy KV bag. Learning Loop keeps learned content:

- isolated via `learning_namespace=learning`
- ranked and filtered by status and recency
- protected by write-gate + dedupe

This reduces "vector junk" and helps governance.

## When to store what

Store in Learning Loop:

- "Steps to do X"
- "Decision record for Y"
- "Snippet for Z"

Store in Culture:

- "User prefers short answers"
- "User prefers English"
- "User likes examples"

## Migration guidance

If you already store factual content in Culture:

1. Move reusable facts into Learning Loop artifacts.
2. Keep only profile/style keys in Culture.
3. Avoid mixing knowledge with user preferences.

## Recommended setup

- Enable Learning Loop for continuous learning.
- Keep Culture enabled for personalization only.
- Use `WithKnowledgeFilters` to scope learning by language, domain, or tenant.


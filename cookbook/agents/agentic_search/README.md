# Agentic Search

This example demonstrates how to build a **Research Agent** that can search the web for information using the `DuckDuckGoTools`.

Agentic Search is a powerful pattern where the agent doesn't rely solely on its internal training data (which might be outdated) but actively retrieves new information from the web to answer queries.

## How it works

1.  **Tool Setup**: We initialize `DuckDuckGoTools` which provides web search capabilities.
2.  **Agent Config**: We give the agent instructions to act as a researcher and "always use the search tool".
3.  **Execution**: When asked about a topic, the agent:
    *   Formulates a search query.
    *   Calls `DuckDuckGo_Search`.
    *   Reads the search results.
    *   Synthesizes a summary based on the retrieved content.

## Running the Example

```bash
go run main.go
```

## Expected Output

```text
=== Agentic Search Example ===
🔎 Researching topic: Latest developments in Quantum Computing 2024
==============================

[Tool Call] DuckDuckGo_Search(query="Latest developments in Quantum Computing 2024")

📝 Research Summary:
Based on the search results, here are the key developments in Quantum Computing in 2024:
...
```

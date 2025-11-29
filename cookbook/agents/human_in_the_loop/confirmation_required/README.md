# Human-in-the-Loop: Confirmation Required

This example demonstrates how to implement a "Human-in-the-Loop" workflow using **Tool Hooks** in Agno.

In many real-world applications, you want an agent to be autonomous but still require human approval for sensitive actions (e.g., deleting files, making payments, posting to social media).

Agno provides `ToolBeforeHooks` which are functions executed *before* a tool is called. If the hook returns an error, the tool execution is blocked.

## How it works

1.  **Define a Hook**: We create a function that prompts the user for confirmation via the console.
2.  **Register the Hook**: We pass this function to the `ToolBeforeHooks` field in `AgentConfig`.
3.  **Intercept**: When the agent tries to call `HackerNews`, the hook triggers.
4.  **Decide**:
    *   If user types `y`: The hook returns `nil`, and the tool executes.
    *   If user types `n`: The hook returns an error, blocking the tool.

## Running the Example

```bash
go run main.go
```

## Expected Output

```text
=== Human-in-the-Loop Example ===
The agent will try to fetch Hacker News stories.
You will be asked to confirm the action.
=================================

🛑 CONFIRMATION REQUIRED
Agent wants to call tool: HackerNews_GetTopStories
Arguments: map[]
Do you want to proceed? (y/n): y
✅ Action confirmed.

🤖 Agent Response:
Here are the top 3 stories from Hacker News:
...
```

# Slack Tool Example

This example demonstrates how to use the comprehensive Slack Tool integration with Agno agents.

## Features

The Slack Tool provides complete integration with Slack's API, including:

### Message Operations
- **send_message**: Send messages to channels with support for blocks and attachments
- **send_thread_reply**: Reply to message threads
- **update_message**: Update existing messages
- **delete_message**: Delete messages

### Channel Operations
- **list_channels**: List all channels in the workspace
- **get_channel_info**: Get detailed information about a channel
- **get_channel_history**: Retrieve message history from a channel
- **create_channel**: Create new public or private channels
- **archive_channel**: Archive channels
- **set_channel_topic**: Set channel topics
- **set_channel_purpose**: Set channel purposes

### User Operations
- **invite_to_channel**: Invite users to channels
- **remove_from_channel**: Remove users from channels
- **get_user_info**: Get detailed user information
- **list_users**: List all users in the workspace
- **get_user_presence**: Check user presence status

### File Operations
- **upload_file**: Upload files to channels
- **list_files**: List files in the workspace
- **delete_file**: Delete files

### Reaction Operations
- **add_reaction**: Add emoji reactions to messages
- **remove_reaction**: Remove emoji reactions from messages
- **get_reactions**: Get all reactions for a message

### Search Operations
- **search_messages**: Search for messages in the workspace
- **search_files**: Search for files in the workspace

### Thread Operations
- **get_thread_replies**: Get all replies in a thread

### Pin Operations
- **pin_message**: Pin messages to channels
- **unpin_message**: Unpin messages from channels
- **list_pins**: List all pinned messages in a channel

## Prerequisites

1. **Slack Bot Token**: You need a Slack Bot Token with appropriate OAuth scopes
2. **Ollama Cloud API Key**: For the AI model

### Creating a Slack App

1. Go to https://api.slack.com/apps
2. Click "Create New App" → "From scratch"
3. Give your app a name and select your workspace
4. Navigate to "OAuth & Permissions"
5. Add the following Bot Token Scopes:
   - `channels:read` - View basic information about public channels
   - `channels:write` - Manage public channels
   - `channels:history` - View messages in public channels
   - `chat:write` - Send messages
   - `files:read` - View files
   - `files:write` - Upload, edit, and delete files
   - `groups:read` - View basic information about private channels
   - `groups:write` - Manage private channels
   - `groups:history` - View messages in private channels
   - `reactions:read` - View emoji reactions
   - `reactions:write` - Add and edit emoji reactions
   - `users:read` - View people in the workspace
   - `users:read.email` - View email addresses of people
   - `pins:read` - View pinned content
   - `pins:write` - Add and remove pinned messages
   - `search:read` - Search workspace content
6. Install the app to your workspace
7. Copy the "Bot User OAuth Token" (starts with `xoxb-`)

## Environment Variables

```bash
export SLACK_BOT_TOKEN="xoxb-your-bot-token-here"
export OLLAMA_API_KEY="your-ollama-api-key"
```

## Running the Example

```bash
cd cookbook/tools/slack_example
go run main.go
```

## Example Usage

### With Agent

```go
// Create Slack tool
slackTool := tools.NewSlackTool(slackToken)

// Create agent with Slack tool
agent, err := agent.NewAgent(agent.AgentConfig{
    Context: ctx,
    Model:   ollamaModel,
    Tools:   []toolkit.Tool{slackTool},
    Instructions: "You are a helpful assistant that can interact with Slack.",
})

// Use natural language
response, err := agent.Run(ctx, "Send a message 'Hello team!' to #general")
```

### Direct Tool Usage

```go
// Create Slack tool
slackTool := tools.NewSlackTool(slackToken)

// Send a message directly
result, err := slackTool.Execute(ctx, map[string]interface{}{
    "action":  "send_message",
    "channel": "C1234567890", // Channel ID
    "text":    "Hello from Agno!",
})

// List channels
result, err := slackTool.Execute(ctx, map[string]interface{}{
    "action": "list_channels",
    "limit":  10,
})

// Get channel history
result, err := slackTool.Execute(ctx, map[string]interface{}{
    "action":  "get_channel_history",
    "channel": "C1234567890",
    "limit":   20,
})

// Add reaction to message
result, err := slackTool.Execute(ctx, map[string]interface{}{
    "action":    "add_reaction",
    "channel":   "C1234567890",
    "timestamp": "1234567890.123456",
    "emoji":     "thumbsup",
})

// Upload file
result, err := slackTool.Execute(ctx, map[string]interface{}{
    "action":   "upload_file",
    "channels": "C1234567890",
    "content":  "File content here",
    "filename": "report.txt",
    "title":    "Monthly Report",
})
```

## Advanced Features

### Thread Management

```go
// Send a message and get timestamp
result, _ := slackTool.Execute(ctx, map[string]interface{}{
    "action":  "send_message",
    "channel": "C1234567890",
    "text":    "Starting a discussion",
})

// Parse result to get timestamp
var response map[string]interface{}
json.Unmarshal([]byte(result), &response)
timestamp := response["timestamp"].(string)

// Reply in thread
slackTool.Execute(ctx, map[string]interface{}{
    "action":    "send_thread_reply",
    "channel":   "C1234567890",
    "thread_ts": timestamp,
    "text":      "This is a reply in the thread",
})

// Get all thread replies
slackTool.Execute(ctx, map[string]interface{}{
    "action":    "get_thread_replies",
    "channel":   "C1234567890",
    "thread_ts": timestamp,
})
```

### Channel Management

```go
// Create a new channel
result, _ := slackTool.Execute(ctx, map[string]interface{}{
    "action":     "create_channel",
    "name":       "project-alpha",
    "is_private": false,
})

// Set channel topic
slackTool.Execute(ctx, map[string]interface{}{
    "action":  "set_channel_topic",
    "channel": "C1234567890",
    "topic":   "Discussion about Project Alpha",
})

// Set channel purpose
slackTool.Execute(ctx, map[string]interface{}{
    "action":  "set_channel_purpose",
    "channel": "C1234567890",
    "purpose": "Coordinate Project Alpha development",
})

// Invite users to channel
slackTool.Execute(ctx, map[string]interface{}{
    "action":  "invite_to_channel",
    "channel": "C1234567890",
    "users":   []string{"U1234567890", "U0987654321"},
})
```

### Search and Discovery

```go
// Search messages
result, _ := slackTool.Execute(ctx, map[string]interface{}{
    "action": "search_messages",
    "query":  "project deadline",
    "count":  20,
})

// Search files
result, _ := slackTool.Execute(ctx, map[string]interface{}{
    "action": "search_files",
    "query":  "report.pdf",
    "count":  10,
})

// List files by user
result, _ := slackTool.Execute(ctx, map[string]interface{}{
    "action": "list_files",
    "user":   "U1234567890",
    "count":  20,
})
```

## Error Handling

All methods return JSON responses with a `success` field:

```json
{
  "success": true,
  "message": "Operation completed successfully",
  "data": { ... }
}
```

On error:
```json
{
  "success": false,
  "error": "Error message here"
}
```

## Best Practices

1. **Rate Limiting**: Slack has rate limits. Implement exponential backoff for retries
2. **Token Security**: Never commit tokens to version control
3. **Scopes**: Request only the OAuth scopes you need
4. **Channel IDs**: Use channel IDs (not names) for reliability
5. **Error Handling**: Always check the `success` field in responses
6. **Timestamps**: Store message timestamps for thread replies and reactions

## Common Use Cases

### 1. Automated Notifications

```go
agent.Run(ctx, "Send a notification to #alerts that the deployment is complete")
```

### 2. Team Coordination

```go
agent.Run(ctx, "Create a channel called 'sprint-planning' and invite @john and @jane")
```

### 3. Information Retrieval

```go
agent.Run(ctx, "Search for messages about 'budget' in the last week")
```

### 4. File Management

```go
agent.Run(ctx, "Upload the quarterly report to #management channel")
```

### 5. User Management

```go
agent.Run(ctx, "Get information about user @john and check if they're online")
```

## Troubleshooting

### "missing_scope" Error
- Check that your bot has the required OAuth scopes
- Reinstall the app to your workspace after adding scopes

### "channel_not_found" Error
- Verify the channel ID is correct
- Ensure the bot is a member of the channel (for private channels)

### "not_authed" or "invalid_auth" Error
- Check that your SLACK_BOT_TOKEN is correct
- Verify the token hasn't been revoked

### "message_not_found" Error
- Verify the message timestamp is correct
- Check that the message hasn't been deleted

## References

- [Slack API Documentation](https://api.slack.com/docs)
- [Slack Bot Token Scopes](https://api.slack.com/scopes)
- [Slack API Methods](https://api.slack.com/methods)
- [Agno Documentation](https://github.com/devalexandre/agno-golang)

## License

This example is part of the Agno project and follows the same license.

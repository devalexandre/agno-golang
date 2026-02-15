---
name: discord
description: "Send messages, react, manage threads/pins, run polls, and moderate Discord channels via the Discord API."
metadata:
  version: "1.0.0"
  requires:
    env: ["DISCORD_BOT_TOKEN"]
---

# Discord Skill

Interact with Discord using the Discord REST API. Requires a bot token.

## Setup

1. Create a bot at https://discord.com/developers/applications
2. Enable MESSAGE CONTENT intent
3. Add bot to server with appropriate permissions
4. Set `DISCORD_BOT_TOKEN` environment variable

## Common Operations

### Send a message

```bash
curl -s -X POST "https://discord.com/api/v10/channels/{channelId}/messages" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello from Agno!"}'
```

### React to a message

```bash
curl -s -X PUT "https://discord.com/api/v10/channels/{channelId}/messages/{messageId}/reactions/%E2%9C%85/@me" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN"
```

### Read recent messages

```bash
curl -s "https://discord.com/api/v10/channels/{channelId}/messages?limit=20" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN"
```

### Edit a message

```bash
curl -s -X PATCH "https://discord.com/api/v10/channels/{channelId}/messages/{messageId}" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content": "Updated text"}'
```

### Delete a message

```bash
curl -s -X DELETE "https://discord.com/api/v10/channels/{channelId}/messages/{messageId}" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN"
```

### Create a thread

```bash
curl -s -X POST "https://discord.com/api/v10/channels/{channelId}/messages/{messageId}/threads" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Discussion Thread"}'
```

### Pin a message

```bash
curl -s -X PUT "https://discord.com/api/v10/channels/{channelId}/pins/{messageId}" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN"
```

### Get guild member info

```bash
curl -s "https://discord.com/api/v10/guilds/{guildId}/members/{userId}" \
  -H "Authorization: Bot $DISCORD_BOT_TOKEN"
```

## Writing Style for Discord

- Keep messages short and conversational (1-3 sentences)
- Use **bold** for emphasis, avoid markdown tables (they render poorly)
- Use emoji for tone
- Break info into multiple quick messages instead of walls of text

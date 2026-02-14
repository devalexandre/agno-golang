---
name: slack
description: "Send messages, react, manage pins, and interact with Slack channels and DMs via the Slack API."
metadata:
  version: "1.0.0"
  requires:
    env: ["SLACK_BOT_TOKEN"]
---

# Slack Skill

Interact with Slack using the Slack Web API. Requires a bot token with appropriate scopes.

## Setup

1. Create a Slack app at https://api.slack.com/apps
2. Add Bot Token Scopes: `chat:write`, `reactions:write`, `reactions:read`, `pins:write`, `pins:read`, `channels:history`, `users:read`
3. Install the app to your workspace
4. Set `SLACK_BOT_TOKEN` environment variable

## Common Operations

### Send a message

```bash
curl -s -X POST "https://slack.com/api/chat.postMessage" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel": "C123", "text": "Hello from Agno!"}'
```

### React to a message

```bash
curl -s -X POST "https://slack.com/api/reactions.add" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel": "C123", "timestamp": "1712023032.1234", "name": "white_check_mark"}'
```

### Read recent messages

```bash
curl -s "https://slack.com/api/conversations.history?channel=C123&limit=20" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN"
```

### Edit a message

```bash
curl -s -X POST "https://slack.com/api/chat.update" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel": "C123", "ts": "1712023032.1234", "text": "Updated text"}'
```

### Delete a message

```bash
curl -s -X POST "https://slack.com/api/chat.delete" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel": "C123", "ts": "1712023032.1234"}'
```

### Pin a message

```bash
curl -s -X POST "https://slack.com/api/pins.add" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"channel": "C123", "timestamp": "1712023032.1234"}'
```

### Get user info

```bash
curl -s "https://slack.com/api/users.info?user=U123" \
  -H "Authorization: Bearer $SLACK_BOT_TOKEN"
```

## Ideas

- React with checkmark to mark completed tasks
- Pin key decisions or weekly status updates
- Send deployment notifications to a channel

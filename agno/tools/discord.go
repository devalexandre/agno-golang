package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const discordBaseURL = "https://discord.com/api/v10"

// DiscordTool provides Discord bot integration for sending messages and managing channels.
type DiscordTool struct {
	toolkit.Toolkit
	botToken   string
	httpClient *http.Client
}

type DiscordSendMessageParams struct {
	ChannelID string `json:"channel_id" description:"The Discord channel ID to send the message to." required:"true"`
	Content   string `json:"content" description:"The message content." required:"true"`
}

type DiscordGetMessagesParams struct {
	ChannelID string `json:"channel_id" description:"The Discord channel ID." required:"true"`
	Limit     int    `json:"limit,omitempty" description:"Number of messages to retrieve (1-100). Default: 10."`
}

type DiscordAddReactionParams struct {
	ChannelID string `json:"channel_id" description:"The Discord channel ID." required:"true"`
	MessageID string `json:"message_id" description:"The message ID." required:"true"`
	Emoji     string `json:"emoji" description:"The emoji to add (Unicode or custom format)." required:"true"`
}

// NewDiscordTool creates a new Discord tool.
// If botToken is empty, it reads from the DISCORD_BOT_TOKEN environment variable.
func NewDiscordTool(botToken string) *DiscordTool {
	if botToken == "" {
		botToken = os.Getenv("DISCORD_BOT_TOKEN")
	}

	t := &DiscordTool{
		botToken:   botToken,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "DiscordTool"
	tk.Description = "Discord bot integration: send messages, read channels, add reactions."

	t.Toolkit = tk
	t.Toolkit.Register("SendMessage", "Send a message to a Discord channel.", t, t.SendMessage, DiscordSendMessageParams{})
	t.Toolkit.Register("GetMessages", "Get recent messages from a Discord channel.", t, t.GetMessages, DiscordGetMessagesParams{})
	t.Toolkit.Register("AddReaction", "Add a reaction to a Discord message.", t, t.AddReaction, DiscordAddReactionParams{})

	return t
}

func (t *DiscordTool) doRequest(method, path string, body interface{}) (interface{}, error) {
	if t.botToken == "" {
		return nil, fmt.Errorf("DISCORD_BOT_TOKEN not set")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, discordBaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+t.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("discord API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if len(respBody) == 0 {
		return map[string]interface{}{"status": "ok"}, nil
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

func (t *DiscordTool) SendMessage(params DiscordSendMessageParams) (interface{}, error) {
	body := map[string]interface{}{
		"content": params.Content,
	}
	return t.doRequest("POST", "/channels/"+params.ChannelID+"/messages", body)
}

func (t *DiscordTool) GetMessages(params DiscordGetMessagesParams) (interface{}, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	path := fmt.Sprintf("/channels/%s/messages?limit=%d", params.ChannelID, limit)
	return t.doRequest("GET", path, nil)
}

func (t *DiscordTool) AddReaction(params DiscordAddReactionParams) (interface{}, error) {
	path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/@me",
		params.ChannelID, params.MessageID, params.Emoji)
	return t.doRequest("PUT", path, nil)
}

func (t *DiscordTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const telegramBaseURL = "https://api.telegram.org/bot"

// TelegramTool provides Telegram bot integration for sending messages and photos.
type TelegramTool struct {
	toolkit.Toolkit
	botToken   string
	httpClient *http.Client
}

type TelegramSendMessageParams struct {
	ChatID string `json:"chat_id" description:"The Telegram chat ID to send the message to." required:"true"`
	Text   string `json:"text" description:"The message text." required:"true"`
}

type TelegramSendPhotoParams struct {
	ChatID   string `json:"chat_id" description:"The Telegram chat ID." required:"true"`
	PhotoURL string `json:"photo_url" description:"URL of the photo to send." required:"true"`
	Caption  string `json:"caption,omitempty" description:"Photo caption text."`
}

type TelegramGetUpdatesParams struct {
	Offset int `json:"offset,omitempty" description:"Identifier of the first update to be returned."`
	Limit  int `json:"limit,omitempty" description:"Number of updates to retrieve (1-100). Default: 10."`
}

// NewTelegramTool creates a new Telegram tool.
// If botToken is empty, it reads from the TELEGRAM_BOT_TOKEN environment variable.
func NewTelegramTool(botToken string) *TelegramTool {
	if botToken == "" {
		botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	}

	t := &TelegramTool{
		botToken:   botToken,
		httpClient: &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "TelegramTool"
	tk.Description = "Telegram bot integration: send messages, photos, and receive updates."

	t.Toolkit = tk
	t.Toolkit.Register("SendMessage", "Send a text message via Telegram.", t, t.SendMessage, TelegramSendMessageParams{})
	t.Toolkit.Register("SendPhoto", "Send a photo via Telegram.", t, t.SendPhoto, TelegramSendPhotoParams{})
	t.Toolkit.Register("GetUpdates", "Get recent bot updates/messages.", t, t.GetUpdates, TelegramGetUpdatesParams{})

	return t
}

func (t *TelegramTool) apiURL(method string) string {
	return telegramBaseURL + t.botToken + "/" + method
}

func (t *TelegramTool) doGet(apiMethod string, params url.Values) (interface{}, error) {
	if t.botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	endpoint := t.apiURL(apiMethod) + "?" + params.Encode()

	resp, err := t.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	return result, nil
}

func (t *TelegramTool) SendMessage(params TelegramSendMessageParams) (interface{}, error) {
	v := url.Values{}
	v.Set("chat_id", params.ChatID)
	v.Set("text", params.Text)
	return t.doGet("sendMessage", v)
}

func (t *TelegramTool) SendPhoto(params TelegramSendPhotoParams) (interface{}, error) {
	v := url.Values{}
	v.Set("chat_id", params.ChatID)
	v.Set("photo", params.PhotoURL)
	if params.Caption != "" {
		v.Set("caption", params.Caption)
	}
	return t.doGet("sendPhoto", v)
}

func (t *TelegramTool) GetUpdates(params TelegramGetUpdatesParams) (interface{}, error) {
	v := url.Values{}
	if params.Offset > 0 {
		v.Set("offset", fmt.Sprintf("%d", params.Offset))
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	v.Set("limit", fmt.Sprintf("%d", limit))
	return t.doGet("getUpdates", v)
}

func (t *TelegramTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

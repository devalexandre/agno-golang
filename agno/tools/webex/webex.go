// Package webex ports Agno's WebexTools: sending messages and listing rooms
// through the Webex REST API (https://developer.webex.com).
package webex

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Webex API base URL.
const DefaultBaseURL = "https://webexapis.com/v1"

// WebexTools talks to Webex rooms.
type WebexTools struct {
	toolkit.Toolkit
	accessToken       string
	baseURL           string
	enableSendMessage bool
	enableListRooms   bool
	all               bool
	httpClient        *http.Client
	client            *restkit.Client
}

// Option configures WebexTools.
type Option func(*WebexTools)

// WithAccessToken sets the access token (default: WEBEX_ACCESS_TOKEN).
func WithAccessToken(s string) Option { return func(w *WebexTools) { w.accessToken = s } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(w *WebexTools) { w.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(w *WebexTools) { w.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(w *WebexTools) { w.httpClient = c } }

// WithEnableSendMessage toggles SendMessage (default true).
func WithEnableSendMessage(v bool) Option { return func(w *WebexTools) { w.enableSendMessage = v } }

// WithEnableListRooms toggles ListRooms (default true).
func WithEnableListRooms(v bool) Option { return func(w *WebexTools) { w.enableListRooms = v } }

// WithAll registers every tool regardless of the enable flags.
func WithAll() Option { return func(w *WebexTools) { w.all = true } }

// SendMessageParams are the parameters of SendMessage.
type SendMessageParams struct {
	RoomID string `json:"room_id" description:"The Room ID to send the message to." required:"true"`
	Text   string `json:"text" description:"The text of the message to send." required:"true"`
}

// ListRoomsParams are the (empty) parameters of ListRooms.
type ListRoomsParams struct{}

// New creates the Webex toolkit.
func New(opts ...Option) *WebexTools {
	w := &WebexTools{baseURL: DefaultBaseURL, enableSendMessage: true, enableListRooms: true}
	w.Toolkit = toolkit.NewToolkit()
	w.Toolkit.Name = "webex"
	w.Toolkit.Description = "Send messages to and list rooms in Webex."
	for _, o := range opts {
		o(w)
	}
	w.accessToken = restkit.Env(w.accessToken, "WEBEX_ACCESS_TOKEN")
	if w.httpClient == nil {
		w.httpClient = w.Toolkit.HTTPClient()
	}
	w.client = &restkit.Client{
		HTTP:    w.httpClient,
		BaseURL: w.baseURL,
		Name:    "Webex API",
		Headers: map[string]string{"Authorization": "Bearer " + w.accessToken},
	}
	if w.all || w.enableSendMessage {
		w.Toolkit.Register("SendMessage", "Send a message to a Webex Room. Returns the Webex response as JSON.", w, w.SendMessage, SendMessageParams{})
	}
	if w.all || w.enableListRooms {
		w.Toolkit.Register("ListRooms", "List all rooms in Webex as JSON.", w, w.ListRooms, ListRoomsParams{})
	}
	return w
}

func (w *WebexTools) check() error {
	if w.accessToken == "" {
		return errors.New("Webex access token is not set. Please set the WEBEX_ACCESS_TOKEN environment variable")
	}
	return nil
}

// SendMessage posts a text message to a room and returns the API response JSON.
func (w *WebexTools) SendMessage(params SendMessageParams) (interface{}, error) {
	if err := w.check(); err != nil {
		return nil, err
	}
	_, data, err := w.client.Raw(http.MethodPost, "/messages", nil, map[string]string{"roomId": params.RoomID, "text": params.Text})
	if err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.CompactJSON(restkit.Decode(data)), nil
}

// ListRooms returns {"rooms": [...]} as JSON.
func (w *WebexTools) ListRooms(params ListRoomsParams) (interface{}, error) {
	if err := w.check(); err != nil {
		return nil, err
	}
	v, err := w.client.JSON(http.MethodGet, "/rooms", nil, nil)
	if err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	m, _ := v.(map[string]interface{})
	rooms := []map[string]interface{}{}
	for _, r := range restkit.Slice(m, "items") {
		rooms = append(rooms, map[string]interface{}{
			"id":         r["id"],
			"title":      r["title"],
			"type":       r["type"],
			"isPublic":   r["isPublic"],
			"isReadOnly": r["isReadOnly"],
		})
	}
	out, _ := json.MarshalIndent(map[string]interface{}{"rooms": rooms}, "", "    ")
	return string(out), nil
}

// Execute implements toolkit.Tool.
func (w *WebexTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return w.Toolkit.Execute(methodName, input)
}

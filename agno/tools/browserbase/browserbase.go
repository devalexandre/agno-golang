// Package browserbase ports Agno's BrowserbaseTools: Browserbase cloud
// browser session management through the REST API
// (https://api.browserbase.com/v1, X-BB-API-Key auth).
//
// Python's navigate_to, screenshot and get_page_content drive the session
// with Playwright over CDP (connect_url); they are not ported because they
// require a CDP/browser client. Sessions created here expose their
// connect_url so a CDP client can attach to them.
package browserbase

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Browserbase API base URL (the SDK default).
const DefaultBaseURL = "https://api.browserbase.com"

// BrowserbaseTools manages Browserbase browser sessions.
type BrowserbaseTools struct {
	toolkit.Toolkit
	apiKey             string
	projectID          string
	baseURL            string
	enableCloseSession bool
	all                bool
	httpClient         *http.Client
	client             *restkit.Client

	mu         sync.Mutex
	sessionID  string
	connectURL string
}

// Option configures BrowserbaseTools.
type Option func(*BrowserbaseTools)

// WithAPIKey sets the API key (default: BROWSERBASE_API_KEY).
func WithAPIKey(v string) Option { return func(b *BrowserbaseTools) { b.apiKey = v } }

// WithProjectID sets the project id (default: BROWSERBASE_PROJECT_ID).
func WithProjectID(v string) Option { return func(b *BrowserbaseTools) { b.projectID = v } }

// WithBaseURL overrides the API endpoint (default: BROWSERBASE_BASE_URL or https://api.browserbase.com).
func WithBaseURL(v string) Option { return func(b *BrowserbaseTools) { b.baseURL = v } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(b *BrowserbaseTools) { b.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(b *BrowserbaseTools) { b.httpClient = c } }

// WithEnableCloseSession toggles CloseSession (default true).
func WithEnableCloseSession(v bool) Option {
	return func(b *BrowserbaseTools) { b.enableCloseSession = v }
}

// WithAll enables every tool.
func WithAll() Option { return func(b *BrowserbaseTools) { b.all = true } }

// EmptyParams is used by methods without arguments.
type EmptyParams struct{}

// ListSessionsParams are the parameters of ListSessions.
type ListSessionsParams struct {
	Status string `json:"status,omitempty" description:"Filter by session status: RUNNING, ERROR, TIMED_OUT or COMPLETED (optional)."`
}

// SessionParams identify a session.
type SessionParams struct {
	SessionID string `json:"session_id" description:"The Browserbase session ID." required:"true"`
}

// CloseSessionParams are the parameters of CloseSession.
type CloseSessionParams struct {
	SessionID string `json:"session_id,omitempty" description:"The session ID to close. Defaults to the session created by this toolkit."`
}

// New creates the Browserbase toolkit.
func New(opts ...Option) *BrowserbaseTools {
	b := &BrowserbaseTools{enableCloseSession: true}
	b.Toolkit = toolkit.NewToolkit()
	b.Toolkit.Name = "browserbase_tools"
	b.Toolkit.Description = "Create and manage Browserbase cloud browser sessions."
	for _, o := range opts {
		o(b)
	}
	b.apiKey = restkit.Env(b.apiKey, "BROWSERBASE_API_KEY")
	b.projectID = restkit.Env(b.projectID, "BROWSERBASE_PROJECT_ID")
	b.baseURL = restkit.Env(b.baseURL, "BROWSERBASE_BASE_URL")
	if b.baseURL == "" {
		b.baseURL = DefaultBaseURL
	}
	if b.httpClient == nil {
		b.httpClient = b.Toolkit.HTTPClient()
	}
	b.client = &restkit.Client{
		HTTP:    b.httpClient,
		BaseURL: b.baseURL,
		Name:    "Browserbase API",
		Headers: map[string]string{"X-BB-API-Key": b.apiKey},
	}
	b.Toolkit.Register("CreateSession", "Create a new Browserbase browser session (reuses the current one if already created). Returns session_id and connect_url.", b, b.CreateSession, EmptyParams{})
	b.Toolkit.Register("ListSessions", "List Browserbase sessions, optionally filtered by status.", b, b.ListSessions, ListSessionsParams{})
	b.Toolkit.Register("GetSession", "Get details of a Browserbase session.", b, b.GetSession, SessionParams{})
	if b.all || b.enableCloseSession {
		b.Toolkit.Register("CloseSession", "Close a Browserbase browser session.", b, b.CloseSession, CloseSessionParams{})
	}
	return b
}

func (b *BrowserbaseTools) check(needProject bool) error {
	if b.apiKey == "" {
		return fmt.Errorf("BROWSERBASE_API_KEY is required. Please set the BROWSERBASE_API_KEY environment variable")
	}
	if needProject && b.projectID == "" {
		return fmt.Errorf("BROWSERBASE_PROJECT_ID is required. Please set the BROWSERBASE_PROJECT_ID environment variable")
	}
	return nil
}

// CreateSession ensures a session exists and returns {session_id, connect_url}.
func (b *BrowserbaseTools) CreateSession(_ EmptyParams) (interface{}, error) {
	if err := b.check(true); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sessionID == "" {
		var resp struct {
			ID         string `json:"id"`
			ConnectURL string `json:"connectUrl"`
		}
		if err := b.client.Into(http.MethodPost, "/v1/sessions", nil, map[string]string{"projectId": b.projectID}, &resp); err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		b.sessionID, b.connectURL = resp.ID, resp.ConnectURL
	}
	return restkit.CompactJSON(map[string]string{"session_id": b.sessionID, "connect_url": b.connectURL}), nil
}

// ListSessions lists sessions.
func (b *BrowserbaseTools) ListSessions(params ListSessionsParams) (interface{}, error) {
	if err := b.check(false); err != nil {
		return nil, err
	}
	var q url.Values
	if params.Status != "" {
		q = url.Values{"status": {params.Status}}
	}
	result, err := b.client.JSON(http.MethodGet, "/v1/sessions", q, nil)
	if err != nil {
		return nil, err
	}
	return restkit.CompactJSON(result), nil
}

// GetSession returns a session's details.
func (b *BrowserbaseTools) GetSession(params SessionParams) (interface{}, error) {
	if err := b.check(false); err != nil {
		return nil, err
	}
	if params.SessionID == "" {
		return restkit.ErrorJSON("session_id is required"), nil
	}
	result, err := b.client.JSON(http.MethodGet, "/v1/sessions/"+url.PathEscape(params.SessionID), nil, nil)
	if err != nil {
		return nil, err
	}
	return restkit.CompactJSON(result), nil
}

// CloseSession requests release of a session and resets local state.
func (b *BrowserbaseTools) CloseSession(params CloseSessionParams) (interface{}, error) {
	b.mu.Lock()
	id := params.SessionID
	if id == "" {
		id = b.sessionID
	}
	if id == b.sessionID {
		b.sessionID, b.connectURL = "", ""
	}
	b.mu.Unlock()

	if id != "" && b.check(true) == nil {
		body := map[string]string{"projectId": b.projectID, "status": "REQUEST_RELEASE"}
		if _, err := b.client.JSON(http.MethodPost, "/v1/sessions/"+url.PathEscape(id), nil, body); err != nil {
			return restkit.CompactJSON(map[string]string{"status": "warning", "message": "Cleanup completed with warning: " + err.Error()}), nil
		}
	}
	return restkit.CompactJSON(map[string]string{
		"status":  "closed",
		"message": "Browser resources cleaned up. Session will auto-close if not already closed.",
	}), nil
}

// Execute implements toolkit.Tool.
func (b *BrowserbaseTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return b.Toolkit.Execute(methodName, input)
}

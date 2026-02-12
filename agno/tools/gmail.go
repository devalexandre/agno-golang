package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const gmailBaseURL = "https://gmail.googleapis.com/gmail/v1/users/me"

// GmailTool provides Gmail integration for reading and sending emails.
// Requires a Google OAuth2 access token with Gmail scopes.
type GmailTool struct {
	toolkit.Toolkit
	accessToken string
	httpClient  *http.Client
}

type GmailSendParams struct {
	To      string `json:"to" description:"Recipient email address." required:"true"`
	Subject string `json:"subject" description:"Email subject." required:"true"`
	Body    string `json:"body" description:"Email body text." required:"true"`
}

type GmailReadParams struct {
	Query      string `json:"query,omitempty" description:"Gmail search query (e.g., 'from:user@example.com', 'is:unread'). Default: 'is:inbox'."`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of emails to return. Default: 5."`
}

type GmailGetParams struct {
	MessageID string `json:"message_id" description:"The Gmail message ID." required:"true"`
}

type GmailCreateDraftParams struct {
	To      string `json:"to" description:"Recipient email address." required:"true"`
	Subject string `json:"subject" description:"Email subject." required:"true"`
	Body    string `json:"body" description:"Email body text." required:"true"`
}

// NewGmailTool creates a new Gmail tool.
// If accessToken is empty, it reads from the GMAIL_ACCESS_TOKEN environment variable.
func NewGmailTool(accessToken string) *GmailTool {
	if accessToken == "" {
		accessToken = os.Getenv("GMAIL_ACCESS_TOKEN")
	}

	t := &GmailTool{
		accessToken: accessToken,
		httpClient:  &http.Client{},
	}

	tk := toolkit.NewToolkit()
	tk.Name = "GmailTool"
	tk.Description = "Gmail integration: send emails, read inbox, get messages, and create drafts."

	t.Toolkit = tk
	t.Toolkit.Register("SendEmail", "Send an email via Gmail.", t, t.SendEmail, GmailSendParams{})
	t.Toolkit.Register("ReadEmails", "Search and list emails from Gmail.", t, t.ReadEmails, GmailReadParams{})
	t.Toolkit.Register("GetEmail", "Get a specific email by ID.", t, t.GetEmail, GmailGetParams{})
	t.Toolkit.Register("CreateDraft", "Create a draft email.", t, t.CreateDraft, GmailCreateDraftParams{})

	return t
}

func (t *GmailTool) doRequest(method, path string, body interface{}) (interface{}, error) {
	if t.accessToken == "" {
		return nil, fmt.Errorf("GMAIL_ACCESS_TOKEN not set. Provide a Google OAuth2 access token")
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, gmailBaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gmail request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gmail API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}

	return result, nil
}

func (t *GmailTool) SendEmail(params GmailSendParams) (interface{}, error) {
	// Gmail API requires base64url-encoded RFC 2822 message
	rawMsg := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		params.To, params.Subject, params.Body)

	encoded := base64.URLEncoding.EncodeToString([]byte(rawMsg))

	body := map[string]interface{}{
		"raw": encoded,
	}

	return t.doRequest("POST", "/messages/send", body)
}

func (t *GmailTool) ReadEmails(params GmailReadParams) (interface{}, error) {
	query := params.Query
	if query == "" {
		query = "is:inbox"
	}
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	path := fmt.Sprintf("/messages?q=%s&maxResults=%d", url.QueryEscape(query), maxResults)
	return t.doRequest("GET", path, nil)
}

func (t *GmailTool) GetEmail(params GmailGetParams) (interface{}, error) {
	return t.doRequest("GET", "/messages/"+params.MessageID, nil)
}

func (t *GmailTool) CreateDraft(params GmailCreateDraftParams) (interface{}, error) {
	rawMsg := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		params.To, params.Subject, params.Body)

	body := map[string]interface{}{
		"message": map[string]interface{}{
			"raw": base64.URLEncoding.EncodeToString([]byte(rawMsg)),
		},
	}

	return t.doRequest("POST", "/drafts", body)
}

func (t *GmailTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

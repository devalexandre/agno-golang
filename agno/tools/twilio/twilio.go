// Package twilio ports Agno's TwilioTools: SMS and call lookups through the
// Twilio REST API (2010-04-01) with HTTP basic auth.
package twilio

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Twilio REST API base URL.
const DefaultBaseURL = "https://api.twilio.com/2010-04-01"

var e164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// TwilioTools sends SMS and inspects calls/messages with Twilio.
type TwilioTools struct {
	toolkit.Toolkit
	accountSID string
	authToken  string
	apiKey     string
	apiSecret  string
	region     string
	edge       string
	baseURL    string

	enableSendSMS        bool
	enableGetCallDetails bool
	enableListMessages   bool
	all                  bool

	httpClient *http.Client
	client     *restkit.Client
}

// Option configures TwilioTools.
type Option func(*TwilioTools)

// WithAccountSID sets the account SID (default: TWILIO_ACCOUNT_SID).
func WithAccountSID(s string) Option { return func(t *TwilioTools) { t.accountSID = s } }

// WithAuthToken sets the auth token (default: TWILIO_AUTH_TOKEN).
func WithAuthToken(s string) Option { return func(t *TwilioTools) { t.authToken = s } }

// WithAPIKey sets the API key (default: TWILIO_API_KEY).
func WithAPIKey(s string) Option { return func(t *TwilioTools) { t.apiKey = s } }

// WithAPISecret sets the API secret (default: TWILIO_API_SECRET).
func WithAPISecret(s string) Option { return func(t *TwilioTools) { t.apiSecret = s } }

// WithRegion sets the Twilio region, e.g. "au1" (default: TWILIO_REGION).
func WithRegion(s string) Option { return func(t *TwilioTools) { t.region = s } }

// WithEdge sets the Twilio edge, e.g. "sydney" (default: TWILIO_EDGE).
func WithEdge(s string) Option { return func(t *TwilioTools) { t.edge = s } }

// WithBaseURL overrides the API base URL (takes precedence over region/edge).
func WithBaseURL(u string) Option { return func(t *TwilioTools) { t.baseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(t *TwilioTools) { t.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *TwilioTools) { t.httpClient = c } }

// WithEnableSendSMS toggles SendSms (default true).
func WithEnableSendSMS(v bool) Option { return func(t *TwilioTools) { t.enableSendSMS = v } }

// WithEnableGetCallDetails toggles GetCallDetails (default true).
func WithEnableGetCallDetails(v bool) Option {
	return func(t *TwilioTools) { t.enableGetCallDetails = v }
}

// WithEnableListMessages toggles ListMessages (default true).
func WithEnableListMessages(v bool) Option { return func(t *TwilioTools) { t.enableListMessages = v } }

// WithAll registers every tool regardless of the enable flags.
func WithAll() Option { return func(t *TwilioTools) { t.all = true } }

// SendSmsParams are the parameters of SendSms.
type SendSmsParams struct {
	To   string `json:"to" description:"Recipient phone number (E.164 format)." required:"true"`
	From string `json:"from_" description:"Sender phone number (must be a Twilio number)." required:"true"`
	Body string `json:"body" description:"Message content." required:"true"`
}

// GetCallDetailsParams are the parameters of GetCallDetails.
type GetCallDetailsParams struct {
	CallSID string `json:"call_sid" description:"The SID of the call to lookup." required:"true"`
}

// ListMessagesParams are the parameters of ListMessages.
type ListMessagesParams struct {
	Limit int `json:"limit,omitempty" description:"Maximum number of messages to return. Default is 20."`
}

// New creates the Twilio toolkit.
func New(opts ...Option) *TwilioTools {
	t := &TwilioTools{enableSendSMS: true, enableGetCallDetails: true, enableListMessages: true}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "twilio"
	t.Toolkit.Description = "Send SMS messages and inspect calls and messages with Twilio."
	for _, o := range opts {
		o(t)
	}
	t.accountSID = restkit.Env(t.accountSID, "TWILIO_ACCOUNT_SID")
	t.authToken = restkit.Env(t.authToken, "TWILIO_AUTH_TOKEN")
	t.apiKey = restkit.Env(t.apiKey, "TWILIO_API_KEY")
	t.apiSecret = restkit.Env(t.apiSecret, "TWILIO_API_SECRET")
	t.region = restkit.Env(t.region, "TWILIO_REGION")
	t.edge = restkit.Env(t.edge, "TWILIO_EDGE")
	if t.baseURL == "" {
		t.baseURL = regionalBaseURL(t.region, t.edge)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	user, pass := t.accountSID, t.authToken
	if t.apiKey != "" && t.apiSecret != "" {
		user, pass = t.apiKey, t.apiSecret
	}
	t.client = &restkit.Client{HTTP: t.httpClient, BaseURL: t.baseURL, Name: "Twilio API", BasicUser: user, BasicPass: pass}

	if t.all || t.enableSendSMS {
		t.Toolkit.Register("SendSms", "Send an SMS message using Twilio. Returns the message SID if successful, an error message otherwise.", t, t.SendSms, SendSmsParams{})
	}
	if t.all || t.enableGetCallDetails {
		t.Toolkit.Register("GetCallDetails", "Get details about a specific call (status, duration, etc).", t, t.GetCallDetails, GetCallDetailsParams{})
	}
	if t.all || t.enableListMessages {
		t.Toolkit.Register("ListMessages", "List recent SMS messages.", t, t.ListMessages, ListMessagesParams{})
	}
	return t
}

// regionalBaseURL builds api[.edge].region.twilio.com like the Python SDK.
func regionalBaseURL(region, edge string) string {
	if region == "" && edge == "" {
		return DefaultBaseURL
	}
	if region == "" {
		region = "us1"
	}
	host := "api"
	if edge != "" {
		host += "." + edge
	}
	return "https://" + host + "." + region + ".twilio.com/2010-04-01"
}

func (t *TwilioTools) check() error {
	if t.accountSID == "" {
		return errors.New("TWILIO_ACCOUNT_SID not set")
	}
	if t.authToken == "" && (t.apiKey == "" || t.apiSecret == "") {
		return errors.New("neither TWILIO_AUTH_TOKEN nor both TWILIO_API_KEY and TWILIO_API_SECRET are set")
	}
	return nil
}

func (t *TwilioTools) accountPath(suffix string) string {
	return "/Accounts/" + url.PathEscape(t.accountSID) + suffix
}

// SendSms sends an SMS and returns "Message sent successfully. SID: ...".
func (t *TwilioTools) SendSms(params SendSmsParams) (interface{}, error) {
	if !e164.MatchString(params.To) {
		return "Error: 'to' number must be in E.164 format (e.g., +1234567890)", nil
	}
	if !e164.MatchString(params.From) {
		return "Error: 'from_' number must be in E.164 format (e.g., +1234567890)", nil
	}
	if strings.TrimSpace(params.Body) == "" {
		return "Error: Message body cannot be empty", nil
	}
	if err := t.check(); err != nil {
		return nil, err
	}
	form := url.Values{"To": {params.To}, "From": {params.From}, "Body": {params.Body}}
	var resp struct {
		SID string `json:"sid"`
	}
	if err := t.client.Into(http.MethodPost, t.accountPath("/Messages.json"), nil, form, &resp); err != nil {
		return fmt.Sprintf("Error sending message: %v", err), nil
	}
	return "Message sent successfully. SID: " + resp.SID, nil
}

// GetCallDetails fetches a call and returns a summary map.
func (t *TwilioTools) GetCallDetails(params GetCallDetailsParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	call, err := t.client.JSON(http.MethodGet, t.accountPath("/Calls/"+url.PathEscape(params.CallSID)+".json"), nil, nil)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}
	m, _ := call.(map[string]interface{})
	return map[string]interface{}{
		"to":         m["to"],
		"from":       m["from"],
		"status":     m["status"],
		"duration":   m["duration"],
		"direction":  m["direction"],
		"price":      m["price"],
		"start_time": pyTime(m["start_time"]),
		"end_time":   pyTime(m["end_time"]),
	}, nil
}

// ListMessages lists recent messages (default limit 20).
func (t *TwilioTools) ListMessages(params ListMessagesParams) (interface{}, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	pageSize := limit
	if pageSize > 1000 {
		pageSize = 1000
	}
	resp, err := t.client.JSON(http.MethodGet, t.accountPath("/Messages.json"), url.Values{"PageSize": {strconv.Itoa(pageSize)}}, nil)
	if err != nil {
		return []map[string]interface{}{{"error": err.Error()}}, nil
	}
	m, _ := resp.(map[string]interface{})
	out := []map[string]interface{}{}
	for _, msg := range restkit.Slice(m, "messages") {
		if len(out) >= limit {
			break
		}
		out = append(out, map[string]interface{}{
			"sid":       msg["sid"],
			"to":        msg["to"],
			"from":      msg["from"],
			"body":      msg["body"],
			"status":    msg["status"],
			"date_sent": pyTime(msg["date_sent"]),
		})
	}
	return out, nil
}

// pyTime renders a Twilio RFC 2822 date like Python's str(datetime).
func pyTime(v interface{}) string {
	s, ok := v.(string)
	if !ok || s == "" {
		return "None"
	}
	if tm, err := time.Parse(time.RFC1123Z, s); err == nil {
		return tm.Format("2006-01-02 15:04:05-07:00")
	}
	return s
}

// Execute implements toolkit.Tool.
func (t *TwilioTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

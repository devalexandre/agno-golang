// Package plivo ports Agno's PlivoTools: SMS, voice calls and number lookups
// through the Plivo REST API (https://www.plivo.com/docs) with basic auth.
package plivo

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

const (
	// DefaultBaseURL is the Plivo REST API base URL.
	DefaultBaseURL = "https://api.plivo.com/v1"
	// DefaultLookupBaseURL is the Plivo Lookup API base URL.
	DefaultLookupBaseURL = "https://lookup.plivo.com/v1"
)

var e164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// PlivoTools sends SMS, places calls and inspects activity with Plivo.
type PlivoTools struct {
	toolkit.Toolkit
	authID        string
	authToken     string
	baseURL       string
	lookupBaseURL string

	enableSendSMS        bool
	enableMakeCall       bool
	enableGetCallDetails bool
	enableListMessages   bool
	enableListCalls      bool
	enableLookupNumber   bool
	all                  bool

	httpClient *http.Client
	client     *restkit.Client
	lookup     *restkit.Client
}

// Option configures PlivoTools.
type Option func(*PlivoTools)

// WithAuthID sets the Auth ID (default: PLIVO_AUTH_ID).
func WithAuthID(s string) Option { return func(p *PlivoTools) { p.authID = s } }

// WithAuthToken sets the Auth Token (default: PLIVO_AUTH_TOKEN).
func WithAuthToken(s string) Option { return func(p *PlivoTools) { p.authToken = s } }

// WithBaseURL overrides the REST API base URL.
func WithBaseURL(u string) Option { return func(p *PlivoTools) { p.baseURL = u } }

// WithLookupBaseURL overrides the Lookup API base URL.
func WithLookupBaseURL(u string) Option { return func(p *PlivoTools) { p.lookupBaseURL = u } }

// WithTimeout sets the HTTP timeout.
func WithTimeout(d time.Duration) Option { return func(p *PlivoTools) { p.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(p *PlivoTools) { p.httpClient = c } }

// WithEnableSendSMS toggles SendSms (default true).
func WithEnableSendSMS(v bool) Option { return func(p *PlivoTools) { p.enableSendSMS = v } }

// WithEnableMakeCall toggles MakeCall (default true).
func WithEnableMakeCall(v bool) Option { return func(p *PlivoTools) { p.enableMakeCall = v } }

// WithEnableGetCallDetails toggles GetCallDetails (default true).
func WithEnableGetCallDetails(v bool) Option {
	return func(p *PlivoTools) { p.enableGetCallDetails = v }
}

// WithEnableListMessages toggles ListMessages (default true).
func WithEnableListMessages(v bool) Option { return func(p *PlivoTools) { p.enableListMessages = v } }

// WithEnableListCalls toggles ListCalls (default true).
func WithEnableListCalls(v bool) Option { return func(p *PlivoTools) { p.enableListCalls = v } }

// WithEnableLookupNumber toggles LookupNumber (default true).
func WithEnableLookupNumber(v bool) Option { return func(p *PlivoTools) { p.enableLookupNumber = v } }

// WithAll registers every tool regardless of the enable flags.
func WithAll() Option { return func(p *PlivoTools) { p.all = true } }

// SendSmsParams are the parameters of SendSms.
type SendSmsParams struct {
	To   string `json:"to" description:"Recipient phone number (E.164 format). Single recipient only." required:"true"`
	From string `json:"from_" description:"Sender ID: a Plivo number, short code, or alphanumeric sender ID." required:"true"`
	Body string `json:"body" description:"Message content." required:"true"`
}

// MakeCallParams are the parameters of MakeCall.
type MakeCallParams struct {
	To           string `json:"to" description:"Recipient phone number (E.164 format)." required:"true"`
	From         string `json:"from_" description:"Caller ID: a Plivo voice-enabled number." required:"true"`
	AnswerURL    string `json:"answer_url" description:"URL Plivo requests when the call is answered; must return Plivo XML." required:"true"`
	AnswerMethod string `json:"answer_method,omitempty" description:"HTTP method Plivo uses for the answer URL, GET or POST (default POST)."`
}

// GetCallDetailsParams are the parameters of GetCallDetails.
type GetCallDetailsParams struct {
	CallUUID string `json:"call_uuid" description:"The UUID of the call to look up." required:"true"`
}

// ListMessagesParams are the parameters of ListMessages.
type ListMessagesParams struct {
	Limit            int    `json:"limit,omitempty" description:"Maximum number of messages to return (capped at 20). Default is 20."`
	Offset           int    `json:"offset,omitempty" description:"Number of messages to skip, for paging past the per-request cap."`
	MessageDirection string `json:"message_direction,omitempty" description:"Filter by direction, 'inbound' or 'outbound' (default all)."`
}

// ListCallsParams are the parameters of ListCalls.
type ListCallsParams struct {
	Limit         int    `json:"limit,omitempty" description:"Maximum number of calls to return (capped at 20). Default is 20."`
	Offset        int    `json:"offset,omitempty" description:"Number of calls to skip, for paging past the per-request cap."`
	CallDirection string `json:"call_direction,omitempty" description:"Filter by direction, 'inbound' or 'outbound' (default all)."`
}

// LookupNumberParams are the parameters of LookupNumber.
type LookupNumberParams struct {
	Number string `json:"number" description:"Phone number to look up (E.164 format)." required:"true"`
}

// New creates the Plivo toolkit.
func New(opts ...Option) *PlivoTools {
	p := &PlivoTools{
		baseURL: DefaultBaseURL, lookupBaseURL: DefaultLookupBaseURL,
		enableSendSMS: true, enableMakeCall: true, enableGetCallDetails: true,
		enableListMessages: true, enableListCalls: true, enableLookupNumber: true,
	}
	p.Toolkit = toolkit.NewToolkit()
	p.Toolkit.Name = "plivo"
	p.Toolkit.Description = "Send SMS, place calls and look up numbers with Plivo."
	for _, o := range opts {
		o(p)
	}
	p.authID = restkit.Env(p.authID, "PLIVO_AUTH_ID")
	p.authToken = restkit.Env(p.authToken, "PLIVO_AUTH_TOKEN")
	if p.httpClient == nil {
		p.httpClient = p.Toolkit.HTTPClient()
	}
	p.client = &restkit.Client{
		HTTP: p.httpClient, Name: "Plivo API", BasicUser: p.authID, BasicPass: p.authToken,
		BaseURL: strings.TrimRight(p.baseURL, "/") + "/Account/" + url.PathEscape(p.authID),
	}
	p.lookup = &restkit.Client{HTTP: p.httpClient, Name: "Plivo Lookup API", BasicUser: p.authID, BasicPass: p.authToken, BaseURL: p.lookupBaseURL}

	reg := func(on bool, name, desc string, fn interface{}, params interface{}) {
		if p.all || on {
			p.Toolkit.Register(name, desc, p, fn, params)
		}
	}
	reg(p.enableSendSMS, "SendSms", "Send an SMS message using Plivo. Returns the message UUID if successful, an error message otherwise.", p.SendSms, SendSmsParams{})
	reg(p.enableMakeCall, "MakeCall", "Place an outbound call using Plivo. Returns the call request UUID if successful, an error message otherwise.", p.MakeCall, MakeCallParams{})
	reg(p.enableGetCallDetails, "GetCallDetails", "Get details about a specific call (state, duration, etc).", p.GetCallDetails, GetCallDetailsParams{})
	reg(p.enableListMessages, "ListMessages", "List recent messages.", p.ListMessages, ListMessagesParams{})
	reg(p.enableListCalls, "ListCalls", "List recent calls.", p.ListCalls, ListCallsParams{})
	reg(p.enableLookupNumber, "LookupNumber", "Look up carrier and line-type information for a phone number.", p.LookupNumber, LookupNumberParams{})
	return p
}

func (p *PlivoTools) check() error {
	if p.authID == "" || p.authToken == "" {
		return errors.New("Plivo credentials not set: PLIVO_AUTH_ID and PLIVO_AUTH_TOKEN are required")
	}
	return nil
}

// SendSms sends an SMS and returns "Message sent successfully. UUID: ...".
func (p *PlivoTools) SendSms(params SendSmsParams) (interface{}, error) {
	if !e164.MatchString(params.To) {
		return "Error: 'to' number must be in E.164 format (e.g., +1234567890)", nil
	}
	if strings.TrimSpace(params.From) == "" {
		return "Error: Sender ID (from_) cannot be empty", nil
	}
	if strings.TrimSpace(params.Body) == "" {
		return "Error: Message body cannot be empty", nil
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	var resp struct {
		MessageUUID []string `json:"message_uuid"`
	}
	body := map[string]string{"src": params.From, "dst": params.To, "text": params.Body}
	if err := p.client.Into(http.MethodPost, "/Message/", nil, body, &resp); err != nil {
		return fmt.Sprintf("Error sending message: %v", err), nil
	}
	id := "unknown"
	if len(resp.MessageUUID) > 0 {
		id = resp.MessageUUID[0]
	}
	return "Message sent successfully. UUID: " + id, nil
}

// MakeCall places an outbound call.
func (p *PlivoTools) MakeCall(params MakeCallParams) (interface{}, error) {
	if !e164.MatchString(params.To) {
		return "Error: 'to' number must be in E.164 format (e.g., +1234567890)", nil
	}
	if strings.TrimSpace(params.From) == "" {
		return "Error: Caller ID (from_) cannot be empty", nil
	}
	if strings.TrimSpace(params.AnswerURL) == "" {
		return "Error: answer_url cannot be empty", nil
	}
	method := strings.ToUpper(params.AnswerMethod)
	if method == "" {
		method = "POST"
	}
	if method != "GET" && method != "POST" {
		return "Error: answer_method must be GET or POST", nil
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	var resp struct {
		RequestUUID string `json:"request_uuid"`
	}
	body := map[string]string{"from": params.From, "to": params.To, "answer_url": params.AnswerURL, "answer_method": method}
	if err := p.client.Into(http.MethodPost, "/Call/", nil, body, &resp); err != nil {
		return fmt.Sprintf("Error placing call: %v", err), nil
	}
	if resp.RequestUUID == "" {
		resp.RequestUUID = "unknown"
	}
	return "Call placed successfully. request_uuid: " + resp.RequestUUID, nil
}

// GetCallDetails fetches a call and returns a summary map.
func (p *PlivoTools) GetCallDetails(params GetCallDetailsParams) (interface{}, error) {
	if err := p.check(); err != nil {
		return nil, err
	}
	v, err := p.client.JSON(http.MethodGet, "/Call/"+url.PathEscape(params.CallUUID)+"/", nil, nil)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}
	c, _ := v.(map[string]interface{})
	return map[string]interface{}{
		"to":              c["to_number"],
		"from":            c["from_number"],
		"state":           c["call_state"],
		"duration":        c["call_duration"],
		"direction":       c["call_direction"],
		"initiation_time": c["initiation_time"],
		"answer_time":     c["answer_time"],
		"total_amount":    c["total_amount"],
	}, nil
}

func pageQuery(limit, offset int, dirKey, dir string) (url.Values, string) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 20 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	q := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	dir = strings.TrimSpace(dir)
	if dir != "" {
		if dir != "inbound" && dir != "outbound" {
			return nil, dirKey + " must be 'inbound' or 'outbound'"
		}
		q.Set(dirKey, dir)
	}
	return q, ""
}

// ListMessages lists recent messages.
func (p *PlivoTools) ListMessages(params ListMessagesParams) (interface{}, error) {
	q, msg := pageQuery(params.Limit, params.Offset, "message_direction", params.MessageDirection)
	if msg != "" {
		return []map[string]interface{}{{"error": msg}}, nil
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	v, err := p.client.JSON(http.MethodGet, "/Message/", q, nil)
	if err != nil {
		return []map[string]interface{}{{"error": err.Error()}}, nil
	}
	m, _ := v.(map[string]interface{})
	out := []map[string]interface{}{}
	for _, o := range restkit.Slice(m, "objects") {
		out = append(out, map[string]interface{}{
			"message_uuid": o["message_uuid"],
			"to":           o["to_number"],
			"from":         o["from_number"],
			"state":        o["message_state"],
			"direction":    o["message_direction"],
			"type":         o["message_type"],
			"message_time": o["message_time"],
		})
	}
	return out, nil
}

// ListCalls lists recent calls.
func (p *PlivoTools) ListCalls(params ListCallsParams) (interface{}, error) {
	q, msg := pageQuery(params.Limit, params.Offset, "call_direction", params.CallDirection)
	if msg != "" {
		return []map[string]interface{}{{"error": msg}}, nil
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	v, err := p.client.JSON(http.MethodGet, "/Call/", q, nil)
	if err != nil {
		return []map[string]interface{}{{"error": err.Error()}}, nil
	}
	m, _ := v.(map[string]interface{})
	out := []map[string]interface{}{}
	for _, o := range restkit.Slice(m, "objects") {
		out = append(out, map[string]interface{}{
			"call_uuid": o["call_uuid"],
			"to":        o["to_number"],
			"from":      o["from_number"],
			"state":     o["call_state"],
			"duration":  o["call_duration"],
			"direction": o["call_direction"],
			"end_time":  o["end_time"],
		})
	}
	return out, nil
}

// LookupNumber returns carrier/country metadata for a number.
func (p *PlivoTools) LookupNumber(params LookupNumberParams) (interface{}, error) {
	if !e164.MatchString(params.Number) {
		return map[string]interface{}{"error": "'number' must be in E.164 format (e.g., +1234567890)"}, nil
	}
	if err := p.check(); err != nil {
		return nil, err
	}
	v, err := p.lookup.JSON(http.MethodGet, "/Number/"+url.PathEscape(params.Number), url.Values{"type": {"carrier"}}, nil)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}, nil
	}
	m, _ := v.(map[string]interface{})
	return map[string]interface{}{
		"number":  m["phone_number"],
		"country": m["country"],
		"format":  m["format"],
		"carrier": m["carrier"],
	}, nil
}

// Execute implements toolkit.Tool.
func (p *PlivoTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return p.Toolkit.Execute(methodName, input)
}

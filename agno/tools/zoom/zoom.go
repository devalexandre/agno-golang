// Package zoom ports Agno's ZoomTools: schedule, list, inspect and delete
// Zoom meetings and fetch recordings through the Zoom API v2, authenticated
// with a Server-to-Server OAuth token (grant_type=account_credentials).
package zoom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const (
	// DefaultBaseURL is the Zoom API v2 base URL.
	DefaultBaseURL = "https://api.zoom.us/v2"
	// DefaultTokenURL is Zoom's OAuth token endpoint.
	DefaultTokenURL = "https://zoom.us/oauth/token"
)

// ZoomTools manages Zoom meetings.
type ZoomTools struct {
	toolkit.Toolkit
	accountID    string
	clientID     string
	clientSecret string
	baseURL      string
	tokenURL     string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// Option configures ZoomTools.
type Option func(*ZoomTools)

// WithAccountID sets the account ID (default: ZOOM_ACCOUNT_ID).
func WithAccountID(id string) Option { return func(z *ZoomTools) { z.accountID = id } }

// WithClientID sets the OAuth client ID (default: ZOOM_CLIENT_ID).
func WithClientID(id string) Option { return func(z *ZoomTools) { z.clientID = id } }

// WithClientSecret sets the OAuth client secret (default: ZOOM_CLIENT_SECRET).
func WithClientSecret(s string) Option { return func(z *ZoomTools) { z.clientSecret = s } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(z *ZoomTools) { z.baseURL = u } }

// WithTokenURL overrides the OAuth token endpoint.
func WithTokenURL(u string) Option { return func(z *ZoomTools) { z.tokenURL = u } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(z *ZoomTools) { z.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(z *ZoomTools) { z.httpClient = c } }

// NoParams is used by methods without arguments.
type NoParams struct{}

// ScheduleMeetingParams are the parameters of ScheduleMeeting.
type ScheduleMeetingParams struct {
	Topic     string `json:"topic" description:"The topic or title of the meeting." required:"true"`
	StartTime string `json:"start_time" description:"The start time of the meeting in ISO 8601 format." required:"true"`
	Duration  int    `json:"duration" description:"The duration of the meeting in minutes." required:"true"`
	Timezone  string `json:"timezone,omitempty" description:"The timezone for the meeting (e.g., 'America/New_York', 'Asia/Tokyo'). Default 'UTC'."`
}

// UserParams identify a user.
type UserParams struct {
	UserID string `json:"user_id,omitempty" description:"The user ID or 'me' for the authenticated user. Defaults to 'me'."`
}

// ListMeetingsParams are the parameters of ListMeetings.
type ListMeetingsParams struct {
	UserID string `json:"user_id,omitempty" description:"The user ID or 'me' for the authenticated user. Defaults to 'me'."`
	Type   string `json:"type,omitempty" description:"The type of meetings to return: 'scheduled', 'live', 'upcoming' or 'previous'. Defaults to 'scheduled'."`
}

// RecordingsParams are the parameters of GetMeetingRecordings.
type RecordingsParams struct {
	MeetingID            string `json:"meeting_id" description:"The meeting ID or UUID to get recordings for." required:"true"`
	IncludeDownloadToken bool   `json:"include_download_token,omitempty" description:"Whether to include download access token in response."`
	TokenTTL             int    `json:"token_ttl,omitempty" description:"Time to live for download token in seconds (max 604800)."`
}

// DeleteMeetingParams are the parameters of DeleteMeeting. When decoded from
// JSON (tool calls), schedule_for_reminder defaults to true as in Python.
type DeleteMeetingParams struct {
	MeetingID           string `json:"meeting_id" description:"The ID of the meeting to delete." required:"true"`
	ScheduleForReminder bool   `json:"schedule_for_reminder" description:"Send cancellation email to registrants. Defaults to True."`
}

// UnmarshalJSON applies Python's default schedule_for_reminder=True.
func (p *DeleteMeetingParams) UnmarshalJSON(b []byte) error {
	type alias DeleteMeetingParams
	a := alias{ScheduleForReminder: true}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*p = DeleteMeetingParams(a)
	return nil
}

// MeetingParams identify a meeting.
type MeetingParams struct {
	MeetingID string `json:"meeting_id" description:"The ID of the meeting to retrieve." required:"true"`
}

// New creates the Zoom toolkit.
func New(opts ...Option) *ZoomTools {
	z := &ZoomTools{baseURL: DefaultBaseURL, tokenURL: DefaultTokenURL}
	z.Toolkit = toolkit.NewToolkit()
	z.Toolkit.Name = "zoom_tool"
	z.Toolkit.Description = "Use this tool to schedule and manage Zoom meetings. You can schedule meetings by providing a topic, start time, and duration."
	for _, o := range opts {
		o(z)
	}
	z.accountID = restkit.Env(z.accountID, "ZOOM_ACCOUNT_ID")
	z.clientID = restkit.Env(z.clientID, "ZOOM_CLIENT_ID")
	z.clientSecret = restkit.Env(z.clientSecret, "ZOOM_CLIENT_SECRET")
	if z.httpClient == nil {
		z.httpClient = z.Toolkit.HTTPClient()
	}
	z.Toolkit.Register("GetAccessToken", "Get a valid access token, refreshing if necessary using Zoom's Server-to-Server OAuth.", z, z.GetAccessToken, NoParams{})
	z.Toolkit.Register("ScheduleMeeting", "Schedule a new Zoom meeting.", z, z.ScheduleMeeting, ScheduleMeetingParams{})
	z.Toolkit.Register("GetUpcomingMeetings", "Get a list of upcoming meetings for a specified user.", z, z.GetUpcomingMeetings, UserParams{})
	z.Toolkit.Register("ListMeetings", "List all meetings for a specified user.", z, z.ListMeetings, ListMeetingsParams{})
	z.Toolkit.Register("GetMeetingRecordings", "Get all recordings for a specific meeting.", z, z.GetMeetingRecordings, RecordingsParams{})
	z.Toolkit.Register("DeleteMeeting", "Delete a scheduled Zoom meeting.", z, z.DeleteMeeting, DeleteMeetingParams{})
	z.Toolkit.Register("GetMeeting", "Get the details of a specific Zoom meeting.", z, z.GetMeeting, MeetingParams{})
	return z
}

func (z *ZoomTools) check() error {
	if z.accountID == "" || z.clientID == "" || z.clientSecret == "" {
		return fmt.Errorf("ZOOM_ACCOUNT_ID, ZOOM_CLIENT_ID, and ZOOM_CLIENT_SECRET must be set either through parameters or environment variables")
	}
	return nil
}

// token returns a cached or freshly generated access token ("" on failure).
func (z *ZoomTools) token() string {
	z.mu.Lock()
	defer z.mu.Unlock()
	if z.accessToken != "" && time.Now().Before(z.tokenExpiry) {
		return z.accessToken
	}
	c := &restkit.Client{HTTP: z.httpClient, Name: "Zoom OAuth", BasicUser: z.clientID, BasicPass: z.clientSecret}
	var resp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	form := url.Values{"grant_type": {"account_credentials"}, "account_id": {z.accountID}}
	if err := c.Into(http.MethodPost, z.tokenURL, nil, form, &resp); err != nil || resp.AccessToken == "" {
		z.accessToken, z.tokenExpiry = "", time.Time{}
		return ""
	}
	z.accessToken = resp.AccessToken
	// Expire slightly before the real expiry, as Python does.
	z.tokenExpiry = time.Now().Add(time.Duration(resp.ExpiresIn-60) * time.Second)
	return z.accessToken
}

// GetAccessToken returns the current access token or "" if generation fails.
func (z *ZoomTools) GetAccessToken(NoParams) (interface{}, error) {
	if err := z.check(); err != nil {
		return nil, err
	}
	return z.token(), nil
}

var errNoToken = restkit.ErrorJSON("Failed to obtain access token")

func (z *ZoomTools) api() (*restkit.Client, string, error) {
	if err := z.check(); err != nil {
		return nil, "", err
	}
	tok := z.token()
	if tok == "" {
		return nil, errNoToken, nil
	}
	return &restkit.Client{HTTP: z.httpClient, BaseURL: z.baseURL, Name: "Zoom API", Headers: map[string]string{"Authorization": "Bearer " + tok}}, "", nil
}

func get(m map[string]interface{}, key string, def interface{}) interface{} {
	if v, ok := m[key]; ok && v != nil {
		return v
	}
	return def
}

func idString(m map[string]interface{}) string {
	switch v := m["id"].(type) {
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func userPath(userID string) string {
	if userID == "" {
		userID = "me"
	}
	return "/users/" + url.PathEscape(userID) + "/meetings"
}

// ScheduleMeeting creates a scheduled meeting for the authenticated user.
func (z *ZoomTools) ScheduleMeeting(p ScheduleMeetingParams) (interface{}, error) {
	c, msg, err := z.api()
	if err != nil || msg != "" {
		return msg, err
	}
	tz := p.Timezone
	if tz == "" {
		tz = "UTC"
	}
	data := map[string]interface{}{
		"topic": p.Topic, "type": 2, "start_time": p.StartTime, "duration": p.Duration, "timezone": tz,
		"settings": map[string]interface{}{
			"host_video": true, "participant_video": true, "join_before_host": false, "mute_upon_entry": false,
			"watermark": true, "audio": "voip", "auto_recording": "none",
		},
	}
	var info map[string]interface{}
	if err := c.Into(http.MethodPost, "/users/me/meetings", nil, data, &info); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]interface{}{
		"message":    "Meeting scheduled successfully!",
		"meeting_id": info["id"],
		"topic":      info["topic"],
		"start_time": info["start_time"],
		"duration":   info["duration"],
		"join_url":   info["join_url"],
	}), nil
}

// GetUpcomingMeetings lists upcoming meetings (page_size 30).
func (z *ZoomTools) GetUpcomingMeetings(p UserParams) (interface{}, error) {
	c, msg, err := z.api()
	if err != nil || msg != "" {
		return msg, err
	}
	var resp map[string]interface{}
	if err := c.Into(http.MethodGet, userPath(p.UserID), url.Values{"type": {"upcoming"}, "page_size": {"30"}}, nil, &resp); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]interface{}{
		"message":  "Upcoming meetings retrieved successfully",
		"meetings": get(resp, "meetings", []interface{}{}),
	}), nil
}

// ListMeetings lists meetings of the given type.
func (z *ZoomTools) ListMeetings(p ListMeetingsParams) (interface{}, error) {
	c, msg, err := z.api()
	if err != nil || msg != "" {
		return msg, err
	}
	typ := p.Type
	if typ == "" {
		typ = "scheduled"
	}
	var resp map[string]interface{}
	if err := c.Into(http.MethodGet, userPath(p.UserID), url.Values{"type": {typ}}, nil, &resp); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]interface{}{
		"message":       "Meetings retrieved successfully",
		"page_count":    get(resp, "page_count", 0),
		"page_number":   get(resp, "page_number", 1),
		"page_size":     get(resp, "page_size", 30),
		"total_records": get(resp, "total_records", 0),
		"meetings":      get(resp, "meetings", []interface{}{}),
	}), nil
}

// GetMeetingRecordings returns the recordings of a meeting.
func (z *ZoomTools) GetMeetingRecordings(p RecordingsParams) (interface{}, error) {
	c, msg, err := z.api()
	if err != nil || msg != "" {
		return msg, err
	}
	q := url.Values{}
	if p.IncludeDownloadToken {
		q.Set("include_fields", "download_access_token")
		if p.TokenTTL > 0 && p.TokenTTL <= 604800 {
			q.Set("ttl", strconv.Itoa(p.TokenTTL))
		}
	}
	var rec map[string]interface{}
	if err := c.Into(http.MethodGet, "/meetings/"+url.PathEscape(p.MeetingID)+"/recordings", q, nil, &rec); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]interface{}{
		"message":         "Meeting recordings retrieved successfully",
		"meeting_id":      idString(rec),
		"uuid":            get(rec, "uuid", ""),
		"host_id":         get(rec, "host_id", ""),
		"topic":           get(rec, "topic", ""),
		"start_time":      get(rec, "start_time", ""),
		"duration":        get(rec, "duration", 0),
		"total_size":      get(rec, "total_size", 0),
		"recording_count": get(rec, "recording_count", 0),
		"recording_files": get(rec, "recording_files", []interface{}{}),
	}), nil
}

// DeleteMeeting deletes a meeting.
func (z *ZoomTools) DeleteMeeting(p DeleteMeetingParams) (interface{}, error) {
	c, msg, err := z.api()
	if err != nil || msg != "" {
		return msg, err
	}
	q := url.Values{"schedule_for_reminder": {strconv.FormatBool(p.ScheduleForReminder)}}
	status, data, err := c.Raw(http.MethodDelete, "/meetings/"+url.PathEscape(p.MeetingID), q, nil)
	if err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	if status == http.StatusNoContent {
		return restkit.JSONString(map[string]interface{}{"message": "Meeting deleted successfully!", "meeting_id": p.MeetingID}), nil
	}
	return restkit.JSONString(restkit.Decode(data)), nil
}

// GetMeeting returns the details of a meeting.
func (z *ZoomTools) GetMeeting(p MeetingParams) (interface{}, error) {
	c, msg, err := z.api()
	if err != nil || msg != "" {
		return msg, err
	}
	var info map[string]interface{}
	if err := c.Into(http.MethodGet, "/meetings/"+url.PathEscape(p.MeetingID), nil, nil, &info); err != nil {
		return restkit.ErrorJSON(err.Error()), nil
	}
	return restkit.JSONString(map[string]interface{}{
		"message":    "Meeting details retrieved successfully",
		"meeting_id": idString(info),
		"topic":      get(info, "topic", ""),
		"type":       get(info, "type", ""),
		"start_time": get(info, "start_time", ""),
		"duration":   get(info, "duration", 0),
		"timezone":   get(info, "timezone", ""),
		"created_at": get(info, "created_at", ""),
		"join_url":   get(info, "join_url", ""),
		"settings":   get(info, "settings", map[string]interface{}{}),
	}), nil
}

// Execute implements toolkit.Tool.
func (z *ZoomTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return z.Toolkit.Execute(methodName, input)
}

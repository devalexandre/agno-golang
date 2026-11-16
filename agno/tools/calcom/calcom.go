// Package calcom ports Agno's CalComTools: available slots, bookings,
// rescheduling and cancellation through the Cal.com API v2
// (Bearer API key, cal-api-version 2024-08-13).
package calcom

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restkit"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Cal.com API v2 base URL.
const DefaultBaseURL = "https://api.cal.com/v2"

// APIVersion is the cal-api-version header sent with every request.
const APIVersion = "2024-08-13"

// CalComTools manages Cal.com bookings.
type CalComTools struct {
	toolkit.Toolkit
	apiKey       string
	eventTypeID  int
	userTimezone string
	baseURL      string
	enabled      map[string]bool
	all          bool
	httpClient   *http.Client
	client       *restkit.Client
}

// Option configures CalComTools.
type Option func(*CalComTools)

// WithAPIKey sets the API key (default: CALCOM_API_KEY).
func WithAPIKey(key string) Option { return func(c *CalComTools) { c.apiKey = key } }

// WithEventTypeID sets the default event type (default: CALCOM_EVENT_TYPE_ID).
func WithEventTypeID(id int) Option { return func(c *CalComTools) { c.eventTypeID = id } }

// WithUserTimezone sets the IANA timezone used for display (default America/New_York).
func WithUserTimezone(tz string) Option { return func(c *CalComTools) { c.userTimezone = tz } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(c *CalComTools) { c.baseURL = u } }

// WithTimeout sets the HTTP timeout (Python default 30s).
func WithTimeout(d time.Duration) Option { return func(c *CalComTools) { c.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *CalComTools) { c.httpClient = h } }

// WithAll enables all methods.
func WithAll() Option { return func(c *CalComTools) { c.all = true } }

func enable(name string, b bool) Option { return func(c *CalComTools) { c.enabled[name] = b } }

// WithEnableGetAvailableSlots toggles GetAvailableSlots (default true).
func WithEnableGetAvailableSlots(b bool) Option { return enable("GetAvailableSlots", b) }

// WithEnableCreateBooking toggles CreateBooking (default true).
func WithEnableCreateBooking(b bool) Option { return enable("CreateBooking", b) }

// WithEnableGetUpcomingBookings toggles GetUpcomingBookings (default true).
func WithEnableGetUpcomingBookings(b bool) Option { return enable("GetUpcomingBookings", b) }

// WithEnableRescheduleBooking toggles RescheduleBooking (default true).
func WithEnableRescheduleBooking(b bool) Option { return enable("RescheduleBooking", b) }

// WithEnableCancelBooking toggles CancelBooking (default true).
func WithEnableCancelBooking(b bool) Option { return enable("CancelBooking", b) }

// SlotsParams are the parameters of GetAvailableSlots.
type SlotsParams struct {
	StartDate string `json:"start_date" description:"Start date in YYYY-MM-DD format." required:"true"`
	EndDate   string `json:"end_date" description:"End date in YYYY-MM-DD format." required:"true"`
}

// CreateBookingParams are the parameters of CreateBooking.
type CreateBookingParams struct {
	StartTime string `json:"start_time" description:"Start time in YYYY-MM-DDTHH:MM:SSZ format." required:"true"`
	Name      string `json:"name" description:"Attendee's name." required:"true"`
	Email     string `json:"email" description:"Attendee's email." required:"true"`
}

// UpcomingParams are the parameters of GetUpcomingBookings.
type UpcomingParams struct {
	Email string `json:"email,omitempty" description:"Attendee's email [Optional]."`
}

// RescheduleParams are the parameters of RescheduleBooking.
type RescheduleParams struct {
	BookingUID   string `json:"booking_uid" description:"Booking UID to reschedule." required:"true"`
	NewStartTime string `json:"new_start_time" description:"New start time in YYYY-MM-DDTHH:MM:SSZ format." required:"true"`
	Reason       string `json:"reason" description:"Reason for rescheduling." required:"true"`
}

// CancelParams are the parameters of CancelBooking.
type CancelParams struct {
	BookingUID string `json:"booking_uid" description:"Booking UID to cancel." required:"true"`
	Reason     string `json:"reason" description:"Reason for cancellation." required:"true"`
}

// New creates the Cal.com toolkit.
func New(opts ...Option) *CalComTools {
	c := &CalComTools{baseURL: DefaultBaseURL, enabled: map[string]bool{}}
	c.Toolkit = toolkit.NewToolkit()
	c.Toolkit.Name = "calcom"
	c.Toolkit.Description = "Check availability and manage bookings on Cal.com."
	for _, o := range opts {
		o(c)
	}
	c.apiKey = restkit.Env(c.apiKey, "CALCOM_API_KEY")
	if c.eventTypeID == 0 {
		c.eventTypeID, _ = strconv.Atoi(restkit.Env("", "CALCOM_EVENT_TYPE_ID"))
	}
	if c.userTimezone == "" {
		c.userTimezone = "America/New_York"
	}
	if c.httpClient == nil {
		c.httpClient = c.Toolkit.HTTPClient()
	}
	c.client = &restkit.Client{
		HTTP:    c.httpClient,
		BaseURL: c.baseURL,
		Name:    "Cal.com API",
		Headers: map[string]string{"Authorization": "Bearer " + c.apiKey, "cal-api-version": APIVersion},
	}
	reg := func(name, desc string, fn, params interface{}) {
		on, set := c.enabled[name]
		if c.all || !set || on {
			c.Toolkit.Register(name, desc, c, fn, params)
		}
	}
	reg("GetAvailableSlots", "Get available time slots for booking.", c.GetAvailableSlots, SlotsParams{})
	reg("CreateBooking", "Create a new booking.", c.CreateBooking, CreateBookingParams{})
	reg("GetUpcomingBookings", "Get all upcoming bookings for an attendee.", c.GetUpcomingBookings, UpcomingParams{})
	reg("RescheduleBooking", "Reschedule an existing booking.", c.RescheduleBooking, RescheduleParams{})
	reg("CancelBooking", "Cancel an existing booking.", c.CancelBooking, CancelParams{})
	return c
}

func (c *CalComTools) check() error {
	if c.apiKey == "" {
		return fmt.Errorf("CALCOM_API_KEY not set. Please set the CALCOM_API_KEY environment variable")
	}
	return nil
}

// do performs a request and returns the status and body; non-2xx responses
// are not errors (callers compare the status like Python does).
func (c *CalComTools) do(method, path string, q url.Values, body interface{}) (int, []byte, error) {
	status, data, err := c.client.Raw(method, path, q, body)
	var apiErr *restkit.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status, []byte(apiErr.Body), nil
	}
	return status, data, err
}

// toUserTZ mirrors Python's _convert_to_user_timezone ("%Y-%m-%d %H:%M %Z").
func (c *CalComTools) toUserTZ(utc string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, utc)
	if err != nil {
		return "", err
	}
	loc, err := time.LoadLocation(c.userTimezone)
	if err != nil {
		return "", err
	}
	return t.In(loc).Format("2006-01-02 15:04 MST"), nil
}

// toUTC mirrors datetime.fromisoformat(s).astimezone(utc).isoformat(timespec="seconds").
func toUTC(s string) (string, error) {
	s = strings.TrimSpace(s)
	var t time.Time
	var err error
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999-07:00"} {
		if t, err = time.Parse(layout, s); err == nil {
			break
		}
	}
	if err != nil {
		// Naive datetimes are interpreted in local time, as Python's astimezone does.
		for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err = time.ParseInLocation(layout, s, time.Local); err == nil {
				break
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("Invalid isoformat string: '%s'", s)
	}
	return t.UTC().Format("2006-01-02T15:04:05") + "+00:00", nil
}

// GetAvailableSlots returns "Available slots: ..." in the user's timezone.
func (c *CalComTools) GetAvailableSlots(p SlotsParams) (interface{}, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	q := url.Values{
		"startTime":   {p.StartDate + "T00:00:00Z"},
		"endTime":     {p.EndDate + "T23:59:59Z"},
		"eventTypeId": {strconv.Itoa(c.eventTypeID)},
	}
	status, data, err := c.do(http.MethodGet, "/slots/available", q, nil)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if status != http.StatusOK {
		return "Failed to fetch slots: " + string(data), nil
	}
	var resp struct {
		Data struct {
			Slots map[string][]struct {
				Time string `json:"time"`
			} `json:"slots"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error: " + err.Error(), nil
	}
	dates := make([]string, 0, len(resp.Data.Slots))
	for d := range resp.Data.Slots {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	slots := []string{}
	for _, d := range dates {
		for _, s := range resp.Data.Slots[d] {
			ut, err := c.toUserTZ(s.Time)
			if err != nil {
				return "Error: " + err.Error(), nil
			}
			slots = append(slots, ut)
		}
	}
	return "Available slots: " + strings.Join(slots, ", "), nil
}

type booking struct {
	UID    string `json:"uid"`
	Start  string `json:"start"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// CreateBooking books the default event type for an attendee.
func (c *CalComTools) CreateBooking(p CreateBookingParams) (interface{}, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	start, err := toUTC(p.StartTime)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	payload := map[string]interface{}{
		"start":       start,
		"eventTypeId": c.eventTypeID,
		"attendee":    map[string]interface{}{"name": p.Name, "email": p.Email, "timeZone": c.userTimezone},
	}
	status, data, err := c.do(http.MethodPost, "/bookings", nil, payload)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if status != http.StatusCreated {
		return "Failed to create booking: " + string(data), nil
	}
	var resp struct {
		Data booking `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error: " + err.Error(), nil
	}
	ut, err := c.toUserTZ(resp.Data.Start)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	return fmt.Sprintf("Booking created successfully for %s. Booking uid: %s", ut, resp.Data.UID), nil
}

// GetUpcomingBookings lists upcoming bookings, optionally for one attendee.
func (c *CalComTools) GetUpcomingBookings(p UpcomingParams) (interface{}, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	q := url.Values{"status": {"upcoming"}}
	if p.Email != "" {
		q.Set("attendeeEmail", p.Email)
	}
	status, data, err := c.do(http.MethodGet, "/bookings", q, nil)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if status != http.StatusOK {
		return "Failed to fetch bookings: " + string(data), nil
	}
	var resp struct {
		Data []booking `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error: " + err.Error(), nil
	}
	if len(resp.Data) == 0 {
		return "No upcoming bookings found.", nil
	}
	info := []string{}
	for _, b := range resp.Data {
		ut, err := c.toUserTZ(b.Start)
		if err != nil {
			return "Error: " + err.Error(), nil
		}
		info = append(info, fmt.Sprintf("uid: %s, Title: %s, Time: %s, Status: %s", b.UID, b.Title, ut, b.Status))
	}
	return "Upcoming bookings:\n" + strings.Join(info, "\n"), nil
}

// RescheduleBooking moves a booking to a new start time.
func (c *CalComTools) RescheduleBooking(p RescheduleParams) (interface{}, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	start, err := toUTC(p.NewStartTime)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	payload := map[string]interface{}{"start": start, "reschedulingReason": p.Reason}
	status, data, err := c.do(http.MethodPost, "/bookings/"+url.PathEscape(p.BookingUID)+"/reschedule", nil, payload)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if status != http.StatusCreated {
		return "Failed to reschedule booking: " + string(data), nil
	}
	var resp struct {
		Data booking `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "Error: " + err.Error(), nil
	}
	ut, err := c.toUserTZ(resp.Data.Start)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	return fmt.Sprintf("Booking rescheduled to %s. New booking uid: %s", ut, resp.Data.UID), nil
}

// CancelBooking cancels a booking.
func (c *CalComTools) CancelBooking(p CancelParams) (interface{}, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	status, data, err := c.do(http.MethodPost, "/bookings/"+url.PathEscape(p.BookingUID)+"/cancel", nil, map[string]interface{}{"cancellationReason": p.Reason})
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	if status != http.StatusOK {
		return "Failed to cancel booking: " + string(data), nil
	}
	return "Booking cancelled successfully.", nil
}

// Execute implements toolkit.Tool.
func (c *CalComTools) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return c.Toolkit.Execute(methodName, input)
}

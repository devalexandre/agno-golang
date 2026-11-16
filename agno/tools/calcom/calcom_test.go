package calcom

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalCom(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" || r.Header.Get("cal-api-version") != APIVersion {
			t.Errorf("bad headers: %v", r.Header)
		}
		body = nil
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &body)
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /slots/available":
			q := r.URL.Query()
			if q.Get("startTime") != "2025-01-06T00:00:00Z" || q.Get("endTime") != "2025-01-07T23:59:59Z" || q.Get("eventTypeId") != "123" {
				t.Errorf("bad slots query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":{"slots":{"2025-01-07":[{"time":"2025-01-07T15:00:00Z"}],"2025-01-06":[{"time":"2025-01-06T14:00:00.000Z"}]}}}`))
		case "POST /bookings":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"data":{"uid":"abc","start":"2025-01-06T14:00:00.000Z"}}`))
		case "POST /bookings/abc/cancel":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"nope"}`))
		}
	}))
	defer srv.Close()

	c := New(WithAPIKey("k"), WithEventTypeID(123), WithUserTimezone("UTC"), WithBaseURL(srv.URL))
	out, _ := c.GetAvailableSlots(SlotsParams{StartDate: "2025-01-06", EndDate: "2025-01-07"})
	if out != "Available slots: 2025-01-06 14:00 UTC, 2025-01-07 15:00 UTC" {
		t.Fatalf("bad slots: %v", out)
	}
	out, _ = c.CreateBooking(CreateBookingParams{StartTime: "2025-01-06T09:00:00-05:00", Name: "Jane", Email: "j@x.com"})
	if out != "Booking created successfully for 2025-01-06 14:00 UTC. Booking uid: abc" {
		t.Fatalf("bad booking: %v", out)
	}
	if body["start"] != "2025-01-06T14:00:00+00:00" || body["eventTypeId"].(float64) != 123 || body["attendee"].(map[string]interface{})["timeZone"] != "UTC" {
		t.Fatalf("bad booking body: %v", body)
	}
	out, _ = c.CancelBooking(CancelParams{BookingUID: "abc", Reason: "x"})
	if out != `Failed to cancel booking: {"error":"nope"}` || body["cancellationReason"] != "x" {
		t.Fatalf("bad cancel: %v", out)
	}
	if len(c.GetMethods()) != 5 || len(New(WithEnableCancelBooking(false)).GetMethods()) != 4 {
		t.Fatal("bad registration")
	}
}

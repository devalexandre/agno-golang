package zoom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestZoom(t *testing.T) {
	tokenCalls := 0
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			tokenCalls++
			user, pass, _ := r.BasicAuth()
			_ = r.ParseForm()
			if r.Method != http.MethodPost || user != "cid" || pass != "sec" || r.PostForm.Get("grant_type") != "account_credentials" || r.PostForm.Get("account_id") != "acc" {
				t.Errorf("bad token request: %s %s %v", user, pass, r.PostForm)
			}
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad auth: %s", r.Header.Get("Authorization"))
		}
		lastQuery = r.URL.RawQuery
		switch r.Method + " " + r.URL.Path {
		case "POST /v2/users/me/meetings":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["topic"] != "Sync" || body["type"].(float64) != 2 || body["timezone"] != "UTC" {
				t.Errorf("bad body: %v", body)
			}
			_, _ = w.Write([]byte(`{"id":123456789,"topic":"Sync","start_time":"2025-01-01T10:00:00Z","duration":30,"join_url":"https://zoom.us/j/1"}`))
		case "GET /v2/meetings/123":
			_, _ = w.Write([]byte(`{"id":123,"topic":"Sync","type":2}`))
		case "DELETE /v2/meetings/123":
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	z := New(WithAccountID("acc"), WithClientID("cid"), WithClientSecret("sec"), WithBaseURL(srv.URL+"/v2"), WithTokenURL(srv.URL+"/oauth/token"))
	out, _ := z.ScheduleMeeting(ScheduleMeetingParams{Topic: "Sync", StartTime: "2025-01-01T10:00:00Z", Duration: 30})
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if res["message"] != "Meeting scheduled successfully!" || res["meeting_id"].(float64) != 123456789 || res["join_url"] != "https://zoom.us/j/1" {
		t.Fatalf("bad schedule: %v", out)
	}
	out, _ = z.GetMeeting(MeetingParams{MeetingID: "123"})
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if res["meeting_id"] != "123" || res["timezone"] != "" {
		t.Fatalf("bad meeting: %v", out)
	}
	// Tool calls get Python's default schedule_for_reminder=true.
	out, err := z.Execute("zoom_tool_DeleteMeeting", json.RawMessage(`{"meeting_id":"123"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if res["message"] != "Meeting deleted successfully!" || lastQuery != "schedule_for_reminder=true" {
		t.Fatalf("bad delete: %v %s", out, lastQuery)
	}
	if tokenCalls != 1 {
		t.Fatalf("token not cached: %d calls", tokenCalls)
	}
}

func TestZoomTokenFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	z := New(WithAccountID("a"), WithClientID("b"), WithClientSecret("c"), WithTokenURL(srv.URL))
	if out, _ := z.GetMeeting(MeetingParams{MeetingID: "1"}); out != `{"error":"Failed to obtain access token"}` {
		t.Fatalf("got %v", out)
	}
}

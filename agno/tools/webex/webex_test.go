package webex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebex(t *testing.T) {
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /messages":
			data, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(data, &body)
			_, _ = w.Write([]byte(`{"id":"msg1","roomId":"R1","text":"hi"}`))
		case "GET /rooms":
			_, _ = w.Write([]byte(`{"items":[{"id":"R1","title":"Team","type":"group","isPublic":false,"isReadOnly":false,"extra":1}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	wx := New(WithAccessToken("tok"), WithBaseURL(srv.URL))
	out, err := wx.SendMessage(SendMessageParams{RoomID: "R1", Text: "hi"})
	if err != nil || !strings.Contains(out.(string), `"id":"msg1"`) || body["roomId"] != "R1" || body["text"] != "hi" {
		t.Fatalf("out=%v err=%v body=%v", out, err, body)
	}
	rooms, _ := wx.ListRooms(ListRoomsParams{})
	var res struct {
		Rooms []map[string]interface{} `json:"rooms"`
	}
	if err := json.Unmarshal([]byte(rooms.(string)), &res); err != nil || len(res.Rooms) != 1 || res.Rooms[0]["title"] != "Team" || res.Rooms[0]["extra"] != nil {
		t.Fatalf("got %v", rooms)
	}
	t.Setenv("WEBEX_ACCESS_TOKEN", "")
	if _, err := New().ListRooms(ListRoomsParams{}); err == nil {
		t.Fatal("expected missing token error")
	}
}

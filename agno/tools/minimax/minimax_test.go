package minimax

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiniMaxGenerateVideo(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/video_generation":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] != "MiniMax-H3" || body["resolution"] != "2K" || body["duration"].(float64) != 5 {
				t.Errorf("bad body %v", body)
			}
			_, _ = w.Write([]byte(`{"task_id":"t1"}`))
		case "/v2/query/video_generation/t1":
			polls++
			if polls == 1 {
				_, _ = w.Write([]byte(`{"task":{"status":"running"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"task":{"status":"succeeded","content":{"url":"https://x/v.mp4"}}}`))
		}
	}))
	defer srv.Close()
	tk := NewMiniMaxTools(WithAPIKey("k"), WithBaseURL(srv.URL+"/v2/video_generation"), WithPollInterval(time.Millisecond), WithMaxWaitTime(5*time.Second))
	out, _ := tk.GenerateVideo(GenerateVideoParams{Prompt: "cat"})
	if !strings.Contains(out.(string), "https://x/v.mp4") || polls != 2 {
		t.Fatalf("got %v polls=%d", out, polls)
	}
	if out, _ := tk.GenerateVideo(GenerateVideoParams{}); out != "Please provide a prompt" {
		t.Fatalf("got %v", out)
	}
}

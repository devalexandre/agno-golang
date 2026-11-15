package fal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFal(t *testing.T) {
	var polls int32
	var lastBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key k" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&lastBody)
			_, _ = w.Write([]byte(`{"request_id":"r1"}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			if atomic.AddInt32(&polls, 1) < 2 {
				_, _ = w.Write([]byte(`{"status":"IN_PROGRESS"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
		case strings.HasPrefix(r.URL.Path, "/fal-ai/flux/requests/r1"):
			_, _ = w.Write([]byte(`{"images":[{"url":"https://x/i.png"}]}`))
		case strings.HasPrefix(r.URL.Path, "/fal-ai/hunyuan-video/requests/r1"):
			_, _ = w.Write([]byte(`{"video":{"url":"https://x/v.mp4"}}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	tk := NewFalTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithPollInterval(time.Millisecond), WithAll())
	out, _ := tk.GenerateMedia(GenerateMediaParams{Prompt: "cat"})
	if out != "Video generated successfully at https://x/v.mp4" || lastBody["prompt"] != "cat" {
		t.Fatal(out, lastBody)
	}
	out, _ = tk.ImageToImage(ImageToImageParams{Prompt: "p", ImageURL: "https://in"})
	if out != "Image generated successfully at https://x/i.png" || lastBody["image_url"] != "https://in" {
		t.Fatal(out)
	}
	if m := tk.Media(); len(m) != 2 || m[0].Kind != "video" {
		t.Fatalf("%+v", m)
	}
}

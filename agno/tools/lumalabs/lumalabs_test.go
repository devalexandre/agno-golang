package lumalabs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestLuma(t *testing.T) {
	var gets int32
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] == "fail" {
				_, _ = w.Write([]byte(`{"id":"g2","state":"queued"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"g1","state":"queued"}`))
			return
		}
		if r.URL.Path == "/generations/g2" {
			_, _ = w.Write([]byte(`{"id":"g2","state":"failed","failure_reason":"nsfw"}`))
			return
		}
		if atomic.AddInt32(&gets, 1) < 2 {
			_, _ = w.Write([]byte(`{"id":"g1","state":"dreaming"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"g1","state":"completed","assets":{"video":"https://x/v.mp4"}}`))
	}))
	defer srv.Close()
	tk := NewLumaLabTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithPollInterval(time.Millisecond))
	out, _ := tk.ImageToVideo(ImageToVideoParams{Prompt: "p", StartImageURL: "https://s", EndImageURL: "https://e"})
	if out != "Video generated successfully: https://x/v.mp4" {
		t.Fatal(out)
	}
	kf := body["keyframes"].(map[string]interface{})
	if body["aspect_ratio"] != "16:9" || len(kf) != 2 {
		t.Fatalf("%v", body)
	}
	out, _ = tk.GenerateVideo(GenerateVideoParams{Prompt: "fail"})
	if out != "Generation failed: nsfw" {
		t.Fatal(out)
	}
	if len(tk.Videos()) != 1 {
		t.Fatal("videos")
	}
}

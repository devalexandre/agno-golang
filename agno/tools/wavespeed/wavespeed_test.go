package wavespeed

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWaveSpeedPolling(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/bytedance/seedream-v5.0-pro":
			_, _ = w.Write([]byte(`{"data":{"id":"p1","status":"created"}}`))
		case r.URL.Path == "/predictions/p1/result":
			polls++
			if polls < 2 {
				_, _ = w.Write([]byte(`{"data":{"id":"p1","status":"processing"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"id":"p1","status":"completed","outputs":["https://x/a.png"]}}`))
		case r.URL.Path == "/bad/model":
			_, _ = w.Write([]byte(`{"data":{"id":"p2","status":"failed","error":"boom"}}`))
		}
	}))
	defer srv.Close()
	tk := NewWaveSpeedTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithPollInterval(time.Millisecond))
	out, _ := tk.GenerateImage(GenerateParams{Prompt: "cat"})
	if !strings.Contains(out.(string), "Generated 1 image(s) successfully: https://x/a.png") {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.GenerateVideo(GenerateParams{Prompt: "cat", Model: "bad/model"})
	if !strings.Contains(out.(string), "Error: prediction failed: boom") {
		t.Fatalf("got %v", out)
	}
}

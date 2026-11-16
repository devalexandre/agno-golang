package aimlapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	polls := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			if r.Header.Get("Authorization") != "" {
				t.Errorf("key leaked to asset download")
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("BIN"))
			return
		}
		if r.Header.Get("Authorization") != "Bearer k" || r.Header.Get("X-Title") != "" {
			t.Errorf("bad headers %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/images/generations":
			_, _ = w.Write([]byte(`{"data":[{"url":"` + srv.URL + `/assets/a.png"}]}`))
		case "/v2/video/generations":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"id":"g1","status":"queued"}`))
				return
			}
			polls++
			if polls == 1 {
				w.WriteHeader(503)
				_, _ = w.Write([]byte(`{"message":"busy"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"g1","status":"completed","video":{"url":"` + srv.URL + `/assets/v.mp4"}}`))
		case "/v1/tts":
			_, _ = w.Write([]byte(`{"audio":{"url":"` + srv.URL + `/assets/s.mp3"}}`))
		case "/v1/stt/create":
			_, _ = w.Write([]byte(`{"generation_id":"s1","status":"queued"}`))
		case "/v1/stt/s1":
			_, _ = w.Write([]byte(`{"status":"completed","result":{"results":{"channels":[{"alternatives":[{"transcript":"hello there"}]}]}}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAIMLAPITools(t *testing.T) {
	srv := newServer(t)
	dir := t.TempDir()
	tk, err := NewAIMLAPITools(WithAPIKey("k"), WithBaseURL(srv.URL+"/v1"), withInsecureDownloads(),
		WithTargetDirectory(dir), WithBaseDir(dir),
		WithVideoPolling(time.Millisecond, time.Second), WithTranscriptionPolling(time.Millisecond, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := tk.GenerateImage(PromptParams{Prompt: "cat"})
	var res struct {
		Images []struct {
			Path     string `json:"path"`
			MimeType string `json:"mime_type"`
		} `json:"images"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil || len(res.Images) != 1 || res.Images[0].MimeType != "image/png" {
		t.Fatalf("got %v", out)
	}
	if out, _ := tk.GenerateVideo(PromptParams{Prompt: "cat"}); !strings.Contains(out.(string), "Video generated successfully.") {
		t.Fatalf("got %v", out)
	}
	if out, _ := tk.GenerateSpeech(GenerateSpeechParams{TextInput: "hi"}); !strings.Contains(out.(string), "Speech generated successfully") {
		t.Fatalf("got %v", out)
	}
	audio := filepath.Join(dir, "a.wav")
	_ = os.WriteFile(audio, []byte("x"), 0o644)
	if out, _ := tk.TranscribeAudio(TranscribeAudioParams{AudioPath: "a.wav"}); out != "hello there" {
		t.Fatalf("got %v", out)
	}
	if out, _ := tk.TranscribeAudio(TranscribeAudioParams{AudioPath: "/etc/passwd"}); !strings.Contains(out.(string), "outside the allowed directory") {
		t.Fatalf("got %v", out)
	}
}

func TestAIMLAPIConstructor(t *testing.T) {
	t.Setenv("AIMLAPI_API_KEY", "")
	if _, err := NewAIMLAPITools(); err == nil {
		t.Fatal("expected missing key error")
	}
	if _, err := NewAIMLAPITools(WithAPIKey("k"), WithSpeech("m", "v", "ogg", 0)); err == nil {
		t.Fatal("expected format error")
	}
}

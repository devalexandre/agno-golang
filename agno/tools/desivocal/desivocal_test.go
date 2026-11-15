package desivocal

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesiVocal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/voices":
			_, _ = w.Write([]byte(`{"v1":{"name":"Asha","audio_gender":"female","voice_type":"standard","languages":["hi","en"],"preview_path":{"hi":"https://x/p.mp3"}}}`))
		case "/generate":
			if r.Header.Get("X_API_KEY") != "k" {
				t.Errorf("missing key")
			}
			_, _ = w.Write([]byte(`{"s3_path":"https://s3/a.mp3"}`))
		}
	}))
	defer srv.Close()
	tk := NewDesiVocalTools(WithAPIKey("k"), WithBaseURL(srv.URL))
	out, _ := tk.GetVoices(GetVoicesParams{})
	if !strings.Contains(out.(string), `"language": "hi, en"`) || !strings.Contains(out.(string), "https://x/p.mp3") {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.TextToSpeech(TextToSpeechParams{Prompt: "namaste"})
	if !strings.Contains(out.(string), "Audio generated successfully: https://s3/a.mp3") {
		t.Fatalf("got %v", out)
	}
}

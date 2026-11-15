package smallest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSmallest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" || r.Header.Get("X-Source") != "agno" {
			t.Errorf("bad headers")
		}
		switch r.URL.Path {
		case "/voices":
			_, _ = w.Write([]byte(`{"voices":[{"voiceId":"v1","displayName":"Kait","tags":{"gender":"female","language":["en"]}}]}`))
		case "/tts":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["voice_id"] != "kaitlyn" || body["model"] != "lightning_v3.1_pro" {
				t.Errorf("bad body %v", body)
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("RIFF"))
		}
	}))
	defer srv.Close()
	tk := NewSmallestTools(WithAPIKey("k"), WithBaseURL(srv.URL+"/tts"), WithVoicesURL(srv.URL+"/voices"), WithTargetDirectory(t.TempDir()))
	out, _ := tk.GetVoices(GetVoicesParams{})
	if !strings.Contains(out.(string), `"id":"v1"`) || !strings.Contains(out.(string), `"gender":"female"`) {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.TextToSpeech(TextToSpeechParams{Prompt: "hello"})
	if !strings.Contains(out.(string), "Audio generated successfully") || !strings.Contains(out.(string), ".wav") {
		t.Fatalf("got %v", out)
	}
}

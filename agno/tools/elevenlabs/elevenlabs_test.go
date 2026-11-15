package elevenlabs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestElevenLabs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("xi-api-key") != "k" {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.URL.Path == "/v1/voices":
			_, _ = w.Write([]byte(`{"voices":[{"voice_id":"v1","name":"Ann","description":"calm"}]}`))
		case strings.HasPrefix(r.URL.Path, "/v1/text-to-speech/vx"):
			if r.URL.Query().Get("output_format") != "pcm_16000" {
				t.Errorf("format %s", r.URL.RawQuery)
			}
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["text"] != "hi" || body["model_id"] != "eleven_multilingual_v2" {
				t.Errorf("body %v", body)
			}
			_, _ = w.Write([]byte("AUDIO"))
		case r.URL.Path == "/v1/sound-generation":
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), `"duration_seconds":2`) {
				t.Errorf("body %s", b)
			}
			_, _ = w.Write([]byte("FX"))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	tk := NewElevenLabsTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithVoiceID("vx"), WithOutputFormat("pcm_16000"), WithTargetDirectory(dir))
	if len(tk.GetMethods()) != 3 {
		t.Fatalf("methods %d", len(tk.GetMethods()))
	}
	out, _ := tk.GetVoices(GetVoicesParams{})
	if !strings.Contains(out.(string), `"id": "v1"`) {
		t.Fatal(out)
	}
	out, _ = tk.TextToSpeech(TextToSpeechParams{Prompt: "hi"})
	if !strings.Contains(out.(string), "Audio generated successfully") || !strings.Contains(out.(string), ".wav") {
		t.Fatal(out)
	}
	out, _ = tk.GenerateSoundEffect(SoundEffectParams{Prompt: "boom", DurationSeconds: 2})
	if !strings.Contains(out.(string), "Sound effect generated") {
		t.Fatal(out)
	}
	a := tk.Audios()
	if len(a) != 2 || string(a[0].Content) != "AUDIO" {
		t.Fatalf("%+v", a)
	}
	if data, _ := os.ReadFile(a[0].Path); string(data) != "AUDIO" {
		t.Fatal("file not saved")
	}
	bad := NewElevenLabsTools(WithAPIKey("bad"), WithBaseURL(srv.URL))
	out, _ = bad.GetVoices(GetVoicesParams{})
	if !strings.HasPrefix(out.(string), "Error:") {
		t.Fatal(out)
	}
}

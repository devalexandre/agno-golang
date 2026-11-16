package cartesia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCartesia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "k" || r.Header.Get("Cartesia-Version") == "" {
			w.WriteHeader(401)
			return
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.URL.Path {
		case "/voices":
			_, _ = w.Write([]byte(`{"data":[{"id":"a","name":"A","description":"d","language":"en"},{"name":"noid"}]}`))
		case "/voices/localize":
			if body["voice_id"] != "base" || body["language"] != "fr" {
				t.Errorf("body %v", body)
			}
			_, _ = w.Write([]byte(`{"id":"new","name":"N"}`))
		case "/tts/bytes":
			if body["model_id"] != "sonic-3.6" || body["voice"].(map[string]interface{})["id"] != "base" {
				t.Errorf("body %v", body)
			}
			_, _ = w.Write([]byte("MP3"))
		}
	}))
	defer srv.Close()
	tk := NewCartesiaTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithDefaultVoiceID("base"), WithAll())
	if len(tk.GetMethods()) != 3 {
		t.Fatal("methods")
	}
	out, _ := tk.ListVoices(ListVoicesParams{})
	if !strings.Contains(out.(string), `"language": "en"`) || strings.Contains(out.(string), "noid") {
		t.Fatal(out)
	}
	out, _ = tk.LocalizeVoice(LocalizeVoiceParams{Name: "N", Description: "d", Language: "fr", OriginalSpeakerGender: "male"})
	if !strings.Contains(out.(string), `"id": "new"`) {
		t.Fatal(out)
	}
	out, _ = tk.TextToSpeech(TextToSpeechParams{Transcript: "hi"})
	if !strings.Contains(out.(string), "attached successfully") || string(tk.Audios()[0].Content) != "MP3" {
		t.Fatal(out)
	}
	if len(NewCartesiaTools(WithAPIKey("k")).GetMethods()) != 2 {
		t.Fatal("default methods")
	}
}

package sixtydb

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSixtyDB(t *testing.T) {
	pcm := []byte{1, 0, 2, 0}
	audioB64 := base64.StdEncoding.EncodeToString(pcmToWAV(pcm))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing auth")
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.URL.Path {
		case "/voices":
			if r.URL.Query().Get("model") != "quality" {
				t.Errorf("bad model")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":[{"voice_id":"v1","name":"Ana","secret":"x"}]}`))
		case "/tts-synthesize":
			_, _ = w.Write([]byte(`{"success":true,"backendResponse":{"audio_base64":"` + audioB64 + `","sample_rate":24000}}`))
		}
	}))
	defer srv.Close()
	if _, err := NewSixtyDBTools(WithAPIKey("k"), WithBaseURL("http://example.com")); err == nil {
		t.Fatal("expected base_url validation error")
	}
	tk, err := NewSixtyDBTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithTargetDirectory(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := tk.GetVoices(GetVoicesParams{})
	if !strings.Contains(out.(string), `"voice_id":"v1"`) || strings.Contains(out.(string), "secret") {
		t.Fatalf("got %v", out)
	}
	if out, _ := tk.TextToSpeech(TextToSpeechParams{Text: "hi"}); out != "Error: provide a workspace voice_id or default_voice_id" {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.TextToSpeech(TextToSpeechParams{Text: "hi", VoiceID: "v1"})
	var res struct {
		Audios []struct{ Path string } `json:"audios"`
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil || len(res.Audios) != 1 {
		t.Fatalf("got %v", out)
	}
	data, _ := os.ReadFile(res.Audios[0].Path)
	if got, err := extractPCM(data); err != nil || string(got) != string(pcm) {
		t.Fatalf("bad wav: %v", err)
	}
}

func TestExtractPCMRejects(t *testing.T) {
	if _, err := extractPCM([]byte("ID3xxx")); err == nil {
		t.Fatal("expected compressed rejection")
	}
	if _, err := extractPCM([]byte{1}); err == nil {
		t.Fatal("expected odd length rejection")
	}
}

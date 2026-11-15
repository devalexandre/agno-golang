package openaitools

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAITools(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing auth")
		}
		switch r.URL.Path {
		case "/audio/transcriptions":
			if err := r.ParseMultipartForm(1 << 20); err != nil || r.FormValue("model") != "whisper-1" {
				t.Errorf("bad multipart: %v", err)
			}
			_, _ = w.Write([]byte("hello world\n"))
		case "/images/generations":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] != "gpt-image-2" || body["response_format"] != nil {
				t.Errorf("unexpected body %v", body)
			}
			_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString([]byte("PNG")) + `"}]}`))
		case "/audio/speech":
			_, _ = w.Write([]byte("MP3DATA"))
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	tk := NewOpenAITools(WithAPIKey("k"), WithBaseURL(srv.URL), WithTargetDirectory(dir))
	if len(tk.GetMethods()) != 3 {
		t.Fatalf("expected 3 methods")
	}
	audio := filepath.Join(dir, "a.mp3")
	_ = os.WriteFile(audio, []byte("x"), 0o644)
	out, _ := tk.TranscribeAudio(TranscribeAudioParams{AudioPath: audio})
	if out != "hello world" {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.GenerateImage(GenerateImageParams{Prompt: "cat"})
	if !strings.Contains(out.(string), "Image generated successfully.") {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.GenerateSpeech(GenerateSpeechParams{TextInput: "hi"})
	var res struct {
		Audios []struct{ Path string } `json:"audios"`
	}
	_ = json.Unmarshal([]byte(out.(string)), &res)
	if len(res.Audios) != 1 {
		t.Fatalf("got %v", out)
	}
	if data, _ := os.ReadFile(res.Audios[0].Path); string(data) != "MP3DATA" {
		t.Fatalf("audio not saved")
	}
}

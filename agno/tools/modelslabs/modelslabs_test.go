package modelslabs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateMediaVideoWithWait(t *testing.T) {
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "k" {
			t.Errorf("missing key")
		}
		switch {
		case r.URL.Path == "/video/text2video":
			if body["model_id"] != "cogvideox" || body["output_type"] != "mp4" {
				t.Errorf("bad payload %v", body)
			}
			_, _ = w.Write([]byte(`{"status":"processing","eta":2,"future_links":["https://x/v.mp4"]}`))
		case strings.HasPrefix(r.URL.Path, "/video/fetch/"):
			fetches++
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}
	}))
	defer srv.Close()
	tk := NewModelsLabTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithWaitForCompletion(true), WithPollInterval(time.Millisecond))
	out, _ := tk.GenerateMedia(GenerateMediaParams{Prompt: "a cat"})
	s := out.(string)
	if !strings.Contains(s, "will be ready in 2 seconds") || !strings.Contains(s, "https://x/v.mp4") || fetches != 1 {
		t.Fatalf("unexpected %s (fetches=%d)", s, fetches)
	}
}

func TestGenerateMediaImageAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","output":["https://x/i.png"]}`))
	}))
	defer srv.Close()
	tk := NewModelsLabTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithFileType(FileTypePNG))
	out, _ := tk.GenerateMedia(GenerateMediaParams{Prompt: "a cat"})
	if !strings.Contains(out.(string), "Image generated successfully (PNG)") {
		t.Fatalf("got %v", out)
	}
	t.Setenv("MODELS_LAB_API_KEY", "")
	if out, _ := NewModelsLabTools().GenerateMedia(GenerateMediaParams{Prompt: "x"}); out != "Please set the MODELS_LAB_API_KEY" {
		t.Fatalf("got %v", out)
	}
}

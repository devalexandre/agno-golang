package nanobanana

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNanoBanana(t *testing.T) {
	img := base64.StdEncoding.EncodeToString([]byte("PNGDATA"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "k" || r.URL.Path != "/models/gemini-2.5-flash-image:generateContent" {
			w.WriteHeader(400)
			return
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		cfg := body["generationConfig"].(map[string]interface{})
		if cfg["imageConfig"].(map[string]interface{})["aspectRatio"] != "16:9" {
			t.Errorf("cfg %v", cfg)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"here"},{"inlineData":{"mimeType":"image/png","data":"` + img + `"}}]}}]}`))
	}))
	defer srv.Close()
	tk, err := NewNanoBananaTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithAspectRatio("16:9"))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := tk.CreateImage(CreateImageParams{Prompt: "banana"})
	if !strings.Contains(out.(string), "here") || !strings.Contains(out.(string), "Image generated successfully") {
		t.Fatal(out)
	}
	if imgs := tk.Images(); len(imgs) != 1 || string(imgs[0].Content) != "PNGDATA" {
		t.Fatal("images")
	}
	if _, err := NewNanoBananaTools(WithAPIKey("k"), WithAspectRatio("7:1")); err == nil {
		t.Fatal("expected ratio error")
	}
	if _, err := NewNanoBananaTools(WithAPIKey("k"), WithModel("x")); err == nil {
		t.Fatal("expected model error")
	}
}

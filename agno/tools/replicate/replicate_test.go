package replicate

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/v1/models/minimax/video-01/predictions":
			_, _ = w.Write([]byte(`{"id":"p1","status":"starting"}`))
		case "/v1/predictions/p1":
			_, _ = w.Write([]byte(`{"id":"p1","status":"succeeded","output":"https://x/out.mp4"}`))
		case "/v1/predictions":
			_, _ = w.Write([]byte(`{"id":"p2","status":"succeeded","output":["https://x/a.png","https://x/b.webp"]}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	tk := NewReplicateTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithPollInterval(time.Millisecond))
	out, _ := tk.GenerateMedia(GenerateMediaParams{Prompt: "x"})
	if out != "Video generated successfully at https://x/out.mp4" {
		t.Fatal(out)
	}
	tk2 := NewReplicateTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithModel("o/m:v1"))
	out, _ = tk2.GenerateMedia(GenerateMediaParams{Prompt: "x"})
	if out != "Image generated successfully at https://x/a.png\nImage generated successfully at https://x/b.webp" {
		t.Fatal(out)
	}
	if out, _ := NewReplicateTools(WithAPIKey("")).GenerateMedia(GenerateMediaParams{}); out != "API key is not set." {
		t.Fatal(out)
	}
}

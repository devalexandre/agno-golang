package twelvelabs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTwelveLabs(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" {
			t.Errorf("missing key")
		}
		switch r.URL.Path {
		case "/analyze":
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model_name"] != "pegasus1.5" || body["video"].(map[string]interface{})["url"] != "https://v" {
				t.Errorf("bad body %v", body)
			}
			_, _ = w.Write([]byte(`{"data":"A cat jumps."}`))
		case "/embed":
			_ = r.ParseMultipartForm(1 << 20)
			if r.FormValue("text") != "cat" {
				t.Errorf("bad text")
			}
			_, _ = w.Write([]byte(`{"text_embedding":{"segments":[{"float":[0.1,0.2,0.3]}]}}`))
		case "/embed/tasks":
			_, _ = w.Write([]byte(`{"_id":"t1"}`))
		case "/embed/tasks/t1/status":
			polls++
			if polls < 2 {
				_, _ = w.Write([]byte(`{"status":"processing"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		case "/embed/tasks/t1":
			_, _ = w.Write([]byte(`{"video_embedding":{"segments":[{"float":[1,2],"start_offset_sec":0,"end_offset_sec":6,"embedding_scope":"clip"}]}}`))
		}
	}))
	defer srv.Close()
	tk := NewTwelveLabsTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithAll(), WithEmbedPollInterval(time.Millisecond))
	if out, _ := tk.AnalyzeVideo(AnalyzeVideoParams{VideoURL: "https://v", Prompt: "what?"}); out != "A cat jumps." {
		t.Fatalf("got %v", out)
	}
	out, _ := tk.EmbedText(EmbedTextParams{Text: "cat"})
	if !strings.Contains(out.(string), `"dimensions":3`) {
		t.Fatalf("got %v", out)
	}
	out, _ = tk.EmbedVideo(EmbedVideoParams{VideoURL: "https://v"})
	if !strings.Contains(out.(string), `"num_segments":1`) || strings.Contains(out.(string), `"float"`) {
		t.Fatalf("got %v", out)
	}
}

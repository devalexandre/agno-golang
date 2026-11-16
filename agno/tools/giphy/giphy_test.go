package giphy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGiphy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("api_key") != "k" {
			w.WriteHeader(403)
			return
		}
		if q.Get("limit") != "2" || q.Get("q") != "cat" {
			t.Errorf("query %v", q)
		}
		_, _ = w.Write([]byte(`{"data":[{"alt_text":"a cat","images":{"original":{"url":"https://g/1.gif"}}}]}`))
	}))
	defer srv.Close()
	tk := NewGiphyTools(WithAPIKey("k"), WithBaseURL(srv.URL), WithLimit(2))
	out, _ := tk.SearchGifs(SearchGifsParams{Query: "cat"})
	if out != "Found 1 GIF(s): ['https://g/1.gif']" || tk.Images()[0].AltText != "a cat" {
		t.Fatal(out)
	}
	out, _ = NewGiphyTools(WithAPIKey("bad"), WithBaseURL(srv.URL)).SearchGifs(SearchGifsParams{Query: "cat"})
	if out != "HTTP error occurred: 403" {
		t.Fatal(out)
	}
}

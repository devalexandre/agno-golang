package unsplash

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const photoJSON = `{"id":"p1","alt_description":"alt","width":10,"urls":{"small":"https://s"},"user":{"name":"Bob","links":{"html":"https://u"}},"tags":[{"title":"a"},{"title":""},{"title":"b"}],"exif":{"make":"Canon"},"location":{"city":"Rio"},"views":5}`

func TestUnsplash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Client-ID k" || r.Header.Get("Accept-Version") != "v1" {
			w.WriteHeader(401)
			return
		}
		q := r.URL.Query()
		switch r.URL.Path {
		case "/search/photos":
			if q.Get("per_page") != "30" || q.Get("page") != "1" || q.Get("color") != "" || q.Get("orientation") != "portrait" {
				t.Errorf("query %v", q)
			}
			_, _ = w.Write([]byte(`{"total":1,"total_pages":1,"results":[` + photoJSON + `]}`))
		case "/photos/p1":
			_, _ = w.Write([]byte(photoJSON))
		case "/photos/random":
			_, _ = w.Write([]byte(`[` + photoJSON + `,` + photoJSON + `]`))
		case "/photos/p1/download":
			_, _ = w.Write([]byte(`{"url":"https://dl"}`))
		}
	}))
	defer srv.Close()
	tk := NewUnsplashTools(WithAccessKey("k"), WithBaseURL(srv.URL), WithAll())
	out, _ := tk.SearchPhotos(SearchPhotosParams{Query: "x", PerPage: 99, Orientation: "portrait", Color: "neon"})
	var res struct {
		Total  float64
		Photos []map[string]interface{}
	}
	if err := json.Unmarshal([]byte(out.(string)), &res); err != nil || res.Total != 1 {
		t.Fatal(out)
	}
	ph := res.Photos[0]
	if ph["description"] != "alt" || len(ph["tags"].([]interface{})) != 2 || ph["author"].(map[string]interface{})["profile_url"] != "https://u" {
		t.Fatalf("%v", ph)
	}
	out, _ = tk.GetPhoto(PhotoIDParams{PhotoID: "p1"})
	var one map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &one)
	if one["exif"].(map[string]interface{})["make"] != "Canon" || one["views"].(float64) != 5 {
		t.Fatal(out)
	}
	out, _ = tk.GetRandomPhoto(RandomPhotoParams{Count: 2})
	_ = json.Unmarshal([]byte(out.(string)), &one)
	if len(one["photos"].([]interface{})) != 2 {
		t.Fatal(out)
	}
	out, _ = tk.DownloadPhoto(PhotoIDParams{PhotoID: "p1"})
	_ = json.Unmarshal([]byte(out.(string)), &one)
	if one["download_url"] != "https://dl" {
		t.Fatal(out)
	}
	if out, _ := NewUnsplashTools(WithAccessKey("")).SearchPhotos(SearchPhotosParams{Query: "x"}); out != noKeyMsg {
		t.Fatal(out)
	}
}

package spotify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"status":401,"message":"Invalid access token"}}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		q := r.URL.Query()
		switch r.Method + " " + r.URL.Path {
		case "GET /search":
			if q.Get("type") == "track" && q.Get("market") != "BR" {
				t.Errorf("market %v", q)
			}
			_, _ = w.Write([]byte(`{"tracks":{"items":[{"id":"t1","name":"Song","artists":[{"name":"A"}],"album":{"name":"Al"},"uri":"spotify:track:t1","popularity":50,"preview_url":null}]},
				"playlists":{"items":[null,{"id":"p1","name":"PL","owner":{"display_name":"me"},"tracks":{"total":3},"external_urls":{"spotify":"https://pl"},"uri":"u"}]}}`))
		case "GET /me":
			_, _ = w.Write([]byte(`{"id":"user1","display_name":"Me"}`))
		case "POST /users/user1/playlists":
			_, _ = w.Write([]byte(`{"id":"np","name":"New","external_urls":{"spotify":"https://np"},"uri":"spotify:playlist:np"}`))
		case "POST /playlists/np/tracks":
			if !strings.Contains(string(body), "spotify:track:t1") {
				t.Errorf("body %s", body)
			}
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"snapshot_id":"s1"}`))
		case "PUT /me/player/play":
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":{"status":404,"reason":"NO_ACTIVE_DEVICE"}}`))
		case "GET /me/player/currently-playing":
			w.WriteHeader(204)
		case "PUT /playlists/np":
			w.WriteHeader(200)
		case "GET /recommendations":
			if q.Get("seed_genres") != "a,b,c,d,e" || q.Get("target_energy") != "0.5" {
				t.Errorf("rec %v", q)
			}
			_, _ = w.Write([]byte(`{"tracks":[]}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
}

func TestSpotify(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()
	tk := NewSpotifyTools(WithAccessToken("tok"), WithBaseURL(srv.URL), WithDefaultMarket("BR"))
	if len(tk.GetMethods()) != 18 {
		t.Fatalf("methods %d", len(tk.GetMethods()))
	}
	out, _ := tk.SearchTracks(SearchMarketParams{Query: "x"})
	var tracks []map[string]interface{}
	_ = json.Unmarshal([]byte(out.(string)), &tracks)
	if len(tracks) != 1 || tracks[0]["album"] != "Al" {
		t.Fatal(out)
	}
	out, _ = tk.SearchPlaylists(SearchParams{Query: "x"})
	if !strings.Contains(out.(string), `"track_count": 3`) {
		t.Fatal(out)
	}
	out, _ = tk.CreatePlaylist(CreatePlaylistParams{Name: "New", TrackURIs: []string{"spotify:track:t1"}})
	if !strings.Contains(out.(string), `"tracks_added": 1`) || !strings.Contains(out.(string), "https://np") {
		t.Fatal(out)
	}
	out, _ = tk.PlayTrack(PlayTrackParams{TrackURI: "spotify:track:t1"})
	if !strings.Contains(out.(string), "No active Spotify device") {
		t.Fatal(out)
	}
	out, _ = tk.GetCurrentlyPlaying(NoParams{})
	if !strings.Contains(out.(string), "Nothing currently playing") {
		t.Fatal(out)
	}
	name := "Renamed"
	out, _ = tk.UpdatePlaylistDetails(UpdatePlaylistParams{PlaylistID: "np", Name: &name})
	if !strings.Contains(out.(string), `"success": true`) {
		t.Fatal(out)
	}
	e := 0.5
	out, _ = tk.GetTrackRecommendations(RecommendationsParams{SeedGenres: []string{"a", "b", "c", "d", "e", "f"}, TargetEnergy: &e})
	if out != "[]" {
		t.Fatal(out)
	}
	out, _ = tk.GetTrackRecommendations(RecommendationsParams{})
	if !strings.Contains(out.(string), "At least one seed") {
		t.Fatal(out)
	}
	bad := NewSpotifyTools(WithAccessToken("x"), WithBaseURL(srv.URL))
	out, _ = bad.GetMyTopTracks(TopParams{})
	if !strings.Contains(out.(string), "Invalid access token") {
		t.Fatal(out)
	}
}

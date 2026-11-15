// Package spotify ports Agno's SpotifyTools: search music, manage playlists and
// control playback through the Spotify Web API (requires an OAuth access token).
package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Spotify Web API base URL.
const DefaultBaseURL = "https://api.spotify.com/v1"

// SpotifyTools wraps the Spotify Web API.
type SpotifyTools struct {
	toolkit.Toolkit
	accessToken   string
	defaultMarket string
	baseURL       string
	httpClient    *http.Client
}

// Option configures SpotifyTools.
type Option func(*SpotifyTools)

// WithAccessToken sets the OAuth access token (default SPOTIFY_ACCESS_TOKEN).
func WithAccessToken(tok string) Option { return func(t *SpotifyTools) { t.accessToken = tok } }

// WithAPIKey is an alias of WithAccessToken.
func WithAPIKey(tok string) Option { return WithAccessToken(tok) }

// WithDefaultMarket sets the default market (default "US").
func WithDefaultMarket(m string) Option { return func(t *SpotifyTools) { t.defaultMarket = m } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *SpotifyTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *SpotifyTools) { t.httpClient = c } }

// WithTimeout sets the request timeout (default 30s).
func WithTimeout(d time.Duration) Option { return func(t *SpotifyTools) { t.Toolkit.Timeout = d } }

// --- parameter structs ---

type NoParams struct{}

type TopParams struct {
	TimeRange string `json:"time_range,omitempty" description:"Time period: short_term (last 4 weeks), medium_term (last 6 months, default), long_term (all time)."`
	Limit     int    `json:"limit,omitempty" description:"Number of items to return (default 20, max 50)."`
}

type SearchParams struct {
	Query      string `json:"query" description:"Search query." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return (max 50)."`
}

type SearchMarketParams struct {
	Query      string `json:"query" description:"Search query." required:"true"`
	MaxResults int    `json:"max_results,omitempty" description:"Maximum number of results to return (default 10, max 50)."`
	Market     string `json:"market,omitempty" description:"Country code for market (e.g., 'US'). Uses default if not specified."`
}

type UserPlaylistsParams struct {
	MaxResults int `json:"max_results,omitempty" description:"Maximum number of playlists to return (default 20, max 50)."`
}

type AlbumTracksParams struct {
	AlbumID string `json:"album_id" description:"The Spotify ID of the album." required:"true"`
	Market  string `json:"market,omitempty" description:"Country code for market (e.g., 'US'). Uses default if not specified."`
}

type ArtistTopTracksParams struct {
	ArtistID string `json:"artist_id" description:"The Spotify ID of the artist." required:"true"`
	Market   string `json:"market,omitempty" description:"Country code for market (e.g., 'US'). Uses default if not specified."`
}

type RecommendationsParams struct {
	SeedTracks         []string `json:"seed_tracks,omitempty" description:"List of Spotify track IDs (not URIs) to use as seeds."`
	SeedArtists        []string `json:"seed_artists,omitempty" description:"List of Spotify artist IDs to use as seeds."`
	SeedGenres         []string `json:"seed_genres,omitempty" description:"List of genres (e.g., pop, hip-hop, rock, electronic)."`
	Limit              int      `json:"limit,omitempty" description:"Number of recommendations to return (default 20, max 100)."`
	TargetEnergy       *float64 `json:"target_energy,omitempty" description:"Target energy level 0.0-1.0 (higher = more energetic)."`
	TargetValence      *float64 `json:"target_valence,omitempty" description:"Target happiness level 0.0-1.0 (higher = happier)."`
	TargetDanceability *float64 `json:"target_danceability,omitempty" description:"Target danceability 0.0-1.0 (higher = more danceable)."`
	TargetTempo        *float64 `json:"target_tempo,omitempty" description:"Target tempo in BPM (e.g., 120)."`
}

type PlayTrackParams struct {
	TrackURI   string `json:"track_uri,omitempty" description:"Spotify URI of track to play (e.g., spotify:track:xxx). If not provided, resumes current playback."`
	ContextURI string `json:"context_uri,omitempty" description:"Spotify URI of context to play (album, artist, playlist)."`
	DeviceID   string `json:"device_id,omitempty" description:"Optional device ID to play on. Uses active device if not specified."`
	PositionMs int    `json:"position_ms,omitempty" description:"Position in milliseconds to start from (default 0)."`
}

type CreatePlaylistParams struct {
	Name        string   `json:"name" description:"Name of the playlist." required:"true"`
	Description string   `json:"description,omitempty" description:"Optional description for the playlist."`
	Public      bool     `json:"public,omitempty" description:"Whether the playlist should be public (default False)."`
	TrackURIs   []string `json:"track_uris,omitempty" description:"Optional list of Spotify track URIs to add initially, e.g. [\"spotify:track:xxx\"]."`
}

type AddTracksParams struct {
	PlaylistID string   `json:"playlist_id" description:"The Spotify ID of the playlist." required:"true"`
	TrackURIs  []string `json:"track_uris" description:"List of Spotify track URIs to add." required:"true"`
	Position   *int     `json:"position,omitempty" description:"Optional position to insert tracks (0-indexed). Appends to end if not specified."`
}

type RemoveTracksParams struct {
	PlaylistID string   `json:"playlist_id" description:"The Spotify ID of the playlist." required:"true"`
	TrackURIs  []string `json:"track_uris" description:"List of Spotify track URIs to remove." required:"true"`
}

type GetPlaylistParams struct {
	PlaylistID    string `json:"playlist_id" description:"The Spotify ID of the playlist." required:"true"`
	IncludeTracks *bool  `json:"include_tracks,omitempty" description:"Whether to include track listing (default True)."`
}

type UpdatePlaylistParams struct {
	PlaylistID  string  `json:"playlist_id" description:"The Spotify ID of the playlist." required:"true"`
	Name        *string `json:"name,omitempty" description:"New name for the playlist (optional)."`
	Description *string `json:"description,omitempty" description:"New description for the playlist (optional)."`
	Public      *bool   `json:"public,omitempty" description:"New visibility setting (optional)."`
}

// NewSpotifyTools creates the toolkit.
func NewSpotifyTools(opts ...Option) *SpotifyTools {
	t := &SpotifyTools{accessToken: os.Getenv("SPOTIFY_ACCESS_TOKEN"), defaultMarket: "US", baseURL: DefaultBaseURL}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "spotify"
	t.Toolkit.Description = "Spotify toolkit for searching songs and managing playlists."
	t.Toolkit.Timeout = 30 * time.Second
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	r := t.Toolkit.Register
	r("SearchTracks", "Search for tracks on Spotify by name, artist, album, or any combination. Returns tracks with id, name, artists, album, uri, and preview_url.", t, t.SearchTracks, SearchMarketParams{})
	r("SearchPlaylists", "Search for playlists on Spotify by name. Use this to find playlists by name before updating them.", t, t.SearchPlaylists, SearchParams{})
	r("SearchArtists", "Search for artists on Spotify. Use this to find an artist's ID before getting their top tracks.", t, t.SearchArtists, SearchParams{})
	r("SearchAlbums", "Search for albums on Spotify. Use this to find an album's ID before getting its tracks.", t, t.SearchAlbums, SearchMarketParams{})
	r("GetUserPlaylists", "Get the current user's playlists (owned or followed).", t, t.GetUserPlaylists, UserPlaylistsParams{})
	r("GetTrackRecommendations", "Get track recommendations based on seed tracks, artists, or genres. Must provide at least one seed; maximum 5 seeds total. Supports target_energy, target_valence, target_danceability (0.0-1.0) and target_tempo (BPM).", t, t.GetTrackRecommendations, RecommendationsParams{})
	r("GetArtistTopTracks", "Get an artist's top tracks on Spotify. Use search_artists first to get the artist_id.", t, t.GetArtistTopTracks, ArtistTopTracksParams{})
	r("GetAlbumTracks", "Get all tracks from an album. Use search_albums first to get the album_id.", t, t.GetAlbumTracks, AlbumTracksParams{})
	r("GetMyTopTracks", "Get the current user's most played tracks. Requires the 'user-top-read' scope.", t, t.GetMyTopTracks, TopParams{})
	r("GetMyTopArtists", "Get the current user's most played artists. Requires the 'user-top-read' scope.", t, t.GetMyTopArtists, TopParams{})
	r("CreatePlaylist", "Create a new playlist for the current user, optionally adding tracks.", t, t.CreatePlaylist, CreatePlaylistParams{})
	r("AddTracksToPlaylist", "Add tracks to an existing playlist.", t, t.AddTracksToPlaylist, AddTracksParams{})
	r("GetPlaylist", "Get details of a playlist and optionally its tracks.", t, t.GetPlaylist, GetPlaylistParams{})
	r("UpdatePlaylistDetails", "Update a playlist's name, description, or visibility.", t, t.UpdatePlaylistDetails, UpdatePlaylistParams{})
	r("RemoveTracksFromPlaylist", "Remove tracks from a playlist.", t, t.RemoveTracksFromPlaylist, RemoveTracksParams{})
	r("GetCurrentUser", "Get the current authenticated user's profile.", t, t.GetCurrentUser, NoParams{})
	r("PlayTrack", "Start or resume playback on the user's Spotify. Requires an active Spotify session and the 'user-modify-playback-state' scope.", t, t.PlayTrack, PlayTrackParams{})
	r("GetCurrentlyPlaying", "Get information about the user's current playback state.", t, t.GetCurrentlyPlaying, NoParams{})
	return t
}

type obj = map[string]interface{}

// makeRequest mirrors Python's _make_request: 204 → {"success": true}; the
// JSON body is returned for any status (Spotify errors carry an "error" key).
func (t *SpotifyTools) makeRequest(method, endpoint string, body interface{}, params url.Values) obj {
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Bearer " + t.accessToken})
	data, err := c.Do(context.Background(), method, endpoint, params, body)
	var apiErr *restapi.APIError
	if err != nil && !errors.As(err, &apiErr) {
		return obj{"error": err.Error()}
	}
	if err == nil && len(data) == 0 { // e.g. 204 No Content
		return obj{"success": true}
	}
	var out obj
	if jerr := json.Unmarshal(data, &out); jerr != nil {
		return obj{"error": fmt.Sprintf("Failed to parse response: %s", string(data))}
	}
	return out
}

func dump(v interface{}) string { return restapi.ToJSON(v) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func orDefault(v, d int) int {
	if v <= 0 {
		return d
	}
	return v
}

func (t *SpotifyTools) market(m string) string {
	if m != "" {
		return m
	}
	return t.defaultMarket
}

func s(v interface{}, keys ...string) interface{} {
	for _, k := range keys {
		mm, ok := v.(obj)
		if !ok {
			return nil
		}
		v = mm[k]
	}
	return v
}

func list(v interface{}, keys ...string) []obj {
	l, _ := s(v, keys...).([]interface{})
	out := make([]obj, 0, len(l))
	for _, it := range l {
		if mm, ok := it.(obj); ok {
			out = append(out, mm)
		}
	}
	return out
}

func names(v interface{}, key string) []interface{} {
	out := []interface{}{}
	for _, a := range list(v, key) {
		out = append(out, a["name"])
	}
	return out
}

func simpleTrack(tr obj, extra ...string) obj {
	o := obj{"id": tr["id"], "name": tr["name"], "artists": names(tr, "artists"), "album": s(tr, "album", "name"), "uri": tr["uri"], "popularity": tr["popularity"]}
	for _, e := range extra {
		o[e] = tr[e]
	}
	return o
}

func simplePlaylist(p obj) obj {
	return obj{"id": p["id"], "name": p["name"], "owner": s(p, "owner", "display_name"), "track_count": s(p, "tracks", "total"),
		"url": s(p, "external_urls", "spotify"), "uri": p["uri"], "public": p["public"]}
}

func simpleArtist(a obj) obj {
	genres := a["genres"]
	if genres == nil {
		genres = []interface{}{}
	}
	return obj{"id": a["id"], "name": a["name"], "genres": genres, "popularity": a["popularity"], "uri": a["uri"], "followers": s(a, "followers", "total")}
}

// GetCurrentUser returns the current user's profile.
func (t *SpotifyTools) GetCurrentUser(NoParams) (interface{}, error) {
	return dump(t.makeRequest(http.MethodGet, "me", nil, nil)), nil
}

func (t *SpotifyTools) top(kind string, p TopParams) obj {
	tr := p.TimeRange
	if tr == "" {
		tr = "medium_term"
	}
	return t.makeRequest(http.MethodGet, "me/top/"+kind, nil, url.Values{"time_range": {tr}, "limit": {strconv.Itoa(min(orDefault(p.Limit, 20), 50))}})
}

// GetMyTopTracks returns the user's most played tracks.
func (t *SpotifyTools) GetMyTopTracks(p TopParams) (interface{}, error) {
	res := t.top("tracks", p)
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for i, tr := range list(res, "items") {
		o := simpleTrack(tr)
		o["rank"] = i + 1
		out = append(out, o)
	}
	return dump(out), nil
}

// GetMyTopArtists returns the user's most played artists.
func (t *SpotifyTools) GetMyTopArtists(p TopParams) (interface{}, error) {
	res := t.top("artists", p)
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for i, a := range list(res, "items") {
		o := simpleArtist(a)
		o["rank"] = i + 1
		out = append(out, o)
	}
	return dump(out), nil
}

func (t *SpotifyTools) search(q, typ string, limit int, market string) obj {
	params := url.Values{"q": {q}, "type": {typ}, "limit": {strconv.Itoa(min(limit, 50))}}
	if market != "" {
		params.Set("market", market)
	}
	return t.makeRequest(http.MethodGet, "search", nil, params)
}

// SearchPlaylists searches playlists.
func (t *SpotifyTools) SearchPlaylists(p SearchParams) (interface{}, error) {
	res := t.search(p.Query, "playlist", orDefault(p.MaxResults, 10), "")
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, pl := range list(res, "playlists", "items") {
		out = append(out, simplePlaylist(pl))
	}
	return dump(out), nil
}

// GetUserPlaylists returns the user's playlists.
func (t *SpotifyTools) GetUserPlaylists(p UserPlaylistsParams) (interface{}, error) {
	res := t.makeRequest(http.MethodGet, "me/playlists", nil, url.Values{"limit": {strconv.Itoa(min(orDefault(p.MaxResults, 20), 50))}})
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, pl := range list(res, "items") {
		out = append(out, simplePlaylist(pl))
	}
	return dump(out), nil
}

// SearchArtists searches artists.
func (t *SpotifyTools) SearchArtists(p SearchParams) (interface{}, error) {
	res := t.search(p.Query, "artist", orDefault(p.MaxResults, 5), "")
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, a := range list(res, "artists", "items") {
		out = append(out, simpleArtist(a))
	}
	return dump(out), nil
}

// SearchAlbums searches albums.
func (t *SpotifyTools) SearchAlbums(p SearchMarketParams) (interface{}, error) {
	res := t.search(p.Query, "album", orDefault(p.MaxResults, 10), t.market(p.Market))
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, a := range list(res, "albums", "items") {
		out = append(out, obj{"id": a["id"], "name": a["name"], "artists": names(a, "artists"), "release_date": a["release_date"],
			"total_tracks": a["total_tracks"], "uri": a["uri"], "album_type": a["album_type"]})
	}
	return dump(out), nil
}

// GetAlbumTracks returns album info and tracks.
func (t *SpotifyTools) GetAlbumTracks(p AlbumTracksParams) (interface{}, error) {
	res := t.makeRequest(http.MethodGet, "albums/"+url.PathEscape(p.AlbumID), nil, url.Values{"market": {t.market(p.Market)}})
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	tracks := []obj{}
	for _, tr := range list(res, "tracks", "items") {
		tracks = append(tracks, obj{"id": tr["id"], "name": tr["name"], "track_number": tr["track_number"], "duration_ms": tr["duration_ms"],
			"uri": tr["uri"], "artists": names(tr, "artists")})
	}
	return dump(obj{
		"album": obj{"id": res["id"], "name": res["name"], "artists": names(res, "artists"), "release_date": res["release_date"],
			"total_tracks": res["total_tracks"], "uri": res["uri"]},
		"tracks": tracks,
	}), nil
}

// GetArtistTopTracks returns an artist's top tracks.
func (t *SpotifyTools) GetArtistTopTracks(p ArtistTopTracksParams) (interface{}, error) {
	res := t.makeRequest(http.MethodGet, "artists/"+url.PathEscape(p.ArtistID)+"/top-tracks", nil, url.Values{"market": {t.market(p.Market)}})
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, tr := range list(res, "tracks") {
		out = append(out, simpleTrack(tr, "preview_url"))
	}
	return dump(out), nil
}

func first5(v []string) string {
	if len(v) > 5 {
		v = v[:5]
	}
	return strings.Join(v, ",")
}

// GetTrackRecommendations returns recommended tracks for the given seeds.
func (t *SpotifyTools) GetTrackRecommendations(p RecommendationsParams) (interface{}, error) {
	params := url.Values{"limit": {strconv.Itoa(min(orDefault(p.Limit, 20), 100))}}
	if len(p.SeedTracks) > 0 {
		params.Set("seed_tracks", first5(p.SeedTracks))
	}
	if len(p.SeedArtists) > 0 {
		params.Set("seed_artists", first5(p.SeedArtists))
	}
	if len(p.SeedGenres) > 0 {
		params.Set("seed_genres", first5(p.SeedGenres))
	}
	for k, v := range map[string]*float64{"target_energy": p.TargetEnergy, "target_valence": p.TargetValence,
		"target_danceability": p.TargetDanceability, "target_tempo": p.TargetTempo} {
		if v != nil {
			params.Set(k, strconv.FormatFloat(*v, 'f', -1, 64))
		}
	}
	if len(p.SeedTracks)+len(p.SeedArtists)+len(p.SeedGenres) == 0 {
		return dump(obj{"error": "At least one seed (tracks, artists, or genres) is required"}), nil
	}
	res := t.makeRequest(http.MethodGet, "recommendations", nil, params)
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, tr := range list(res, "tracks") {
		out = append(out, simpleTrack(tr, "preview_url"))
	}
	return dump(out), nil
}

// PlayTrack starts or resumes playback.
func (t *SpotifyTools) PlayTrack(p PlayTrackParams) (interface{}, error) {
	var params url.Values
	if p.DeviceID != "" {
		params = url.Values{"device_id": {p.DeviceID}}
	}
	body := obj{}
	if p.TrackURI != "" {
		body["uris"] = []string{p.TrackURI}
	}
	if p.ContextURI != "" {
		body["context_uri"] = p.ContextURI
	}
	if p.PositionMs != 0 {
		body["position_ms"] = p.PositionMs
	}
	var b interface{}
	if len(body) > 0 {
		b = body
	}
	res := t.makeRequest(http.MethodPut, "me/player/play", b, params)
	if res["success"] == true || res["error"] == nil {
		return dump(obj{"success": true, "message": "Playback started"}), nil
	}
	if s(res, "error", "reason") == "NO_ACTIVE_DEVICE" {
		return dump(obj{"error": "No active Spotify device found. Please open Spotify on any device first.", "reason": "NO_ACTIVE_DEVICE"}), nil
	}
	return dump(res), nil
}

// GetCurrentlyPlaying returns the current playback state.
func (t *SpotifyTools) GetCurrentlyPlaying(NoParams) (interface{}, error) {
	res := t.makeRequest(http.MethodGet, "me/player/currently-playing", nil, nil)
	if len(res) == 0 || res["success"] == true {
		return dump(obj{"message": "Nothing currently playing"}), nil
	}
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	var track interface{}
	if item, ok := res["item"].(obj); ok && len(item) > 0 {
		track = obj{"id": item["id"], "name": item["name"], "artists": names(item, "artists"), "album": s(item, "album", "name"),
			"uri": item["uri"], "duration_ms": item["duration_ms"]}
	}
	return dump(obj{"is_playing": res["is_playing"], "progress_ms": res["progress_ms"], "track": track, "device": s(res, "device", "name")}), nil
}

// SearchTracks searches tracks.
func (t *SpotifyTools) SearchTracks(p SearchMarketParams) (interface{}, error) {
	res := t.search(p.Query, "track", orDefault(p.MaxResults, 10), t.market(p.Market))
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	out := []obj{}
	for _, tr := range list(res, "tracks", "items") {
		out = append(out, simpleTrack(tr, "preview_url"))
	}
	return dump(out), nil
}

func capURIs(u []string) []string {
	if len(u) > 100 {
		return u[:100]
	}
	return u
}

// CreatePlaylist creates a playlist for the current user.
func (t *SpotifyTools) CreatePlaylist(p CreatePlaylistParams) (interface{}, error) {
	user := t.makeRequest(http.MethodGet, "me", nil, nil)
	if _, ok := user["error"]; ok {
		return dump(user), nil
	}
	userID, _ := user["id"].(string)
	pl := t.makeRequest(http.MethodPost, "users/"+url.PathEscape(userID)+"/playlists", obj{"name": p.Name, "description": p.Description, "public": p.Public}, nil)
	if _, ok := pl["error"]; ok {
		return dump(pl), nil
	}
	added := 0
	if len(p.TrackURIs) > 0 {
		uris := capURIs(p.TrackURIs)
		pid, _ := pl["id"].(string)
		add := t.makeRequest(http.MethodPost, "playlists/"+url.PathEscape(pid)+"/tracks", obj{"uris": uris}, nil)
		if e, ok := add["error"]; ok {
			pl["track_add_error"] = e
		} else {
			added = len(uris)
		}
	}
	return dump(obj{"id": pl["id"], "name": pl["name"], "description": pl["description"], "url": s(pl, "external_urls", "spotify"),
		"uri": pl["uri"], "tracks_added": added}), nil
}

// AddTracksToPlaylist adds tracks to a playlist.
func (t *SpotifyTools) AddTracksToPlaylist(p AddTracksParams) (interface{}, error) {
	uris := capURIs(p.TrackURIs)
	body := obj{"uris": uris}
	if p.Position != nil {
		body["position"] = *p.Position
	}
	res := t.makeRequest(http.MethodPost, "playlists/"+url.PathEscape(p.PlaylistID)+"/tracks", body, nil)
	if snap, ok := res["snapshot_id"]; ok {
		return dump(obj{"success": true, "tracks_added": len(uris), "snapshot_id": snap}), nil
	}
	return dump(res), nil
}

// RemoveTracksFromPlaylist removes tracks from a playlist.
func (t *SpotifyTools) RemoveTracksFromPlaylist(p RemoveTracksParams) (interface{}, error) {
	tracks := make([]obj, 0, len(p.TrackURIs))
	for _, u := range p.TrackURIs {
		tracks = append(tracks, obj{"uri": u})
	}
	res := t.makeRequest(http.MethodDelete, "playlists/"+url.PathEscape(p.PlaylistID)+"/tracks", obj{"tracks": tracks}, nil)
	if snap, ok := res["snapshot_id"]; ok {
		return dump(obj{"success": true, "tracks_removed": len(p.TrackURIs), "snapshot_id": snap}), nil
	}
	return dump(res), nil
}

// GetPlaylist returns playlist details.
func (t *SpotifyTools) GetPlaylist(p GetPlaylistParams) (interface{}, error) {
	include := p.IncludeTracks == nil || *p.IncludeTracks
	fields := "id,name,description,public,owner(display_name),external_urls"
	if include {
		fields += ",tracks.items(track(id,name,artists(name),uri))"
	}
	res := t.makeRequest(http.MethodGet, "playlists/"+url.PathEscape(p.PlaylistID), nil, url.Values{"fields": {fields}})
	if _, ok := res["error"]; ok {
		return dump(res), nil
	}
	info := obj{"id": res["id"], "name": res["name"], "description": res["description"], "public": res["public"],
		"owner": s(res, "owner", "display_name"), "url": s(res, "external_urls", "spotify")}
	if _, ok := res["tracks"]; include && ok {
		tracks := []obj{}
		for _, item := range list(res, "tracks", "items") {
			tr, ok := item["track"].(obj)
			if !ok {
				continue
			}
			tracks = append(tracks, obj{"id": tr["id"], "name": tr["name"], "artists": names(tr, "artists"), "uri": tr["uri"]})
		}
		info["tracks"] = tracks
	}
	return dump(info), nil
}

// UpdatePlaylistDetails updates name, description or visibility.
func (t *SpotifyTools) UpdatePlaylistDetails(p UpdatePlaylistParams) (interface{}, error) {
	body := obj{}
	var keys []string
	if p.Name != nil {
		body["name"] = *p.Name
		keys = append(keys, "name")
	}
	if p.Description != nil {
		body["description"] = *p.Description
		keys = append(keys, "description")
	}
	if p.Public != nil {
		body["public"] = *p.Public
		keys = append(keys, "public")
	}
	if len(body) == 0 {
		return dump(obj{"error": "No updates provided"}), nil
	}
	res := t.makeRequest(http.MethodPut, "playlists/"+url.PathEscape(p.PlaylistID), body, nil)
	if _, hasErr := res["error"]; res["success"] == true || !hasErr {
		return dump(obj{"success": true, "updated_fields": keys}), nil
	}
	return dump(res), nil
}

// Package unsplash ports Agno's UnsplashTools: search and retrieve royalty-free
// photos from the Unsplash API.
package unsplash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the Unsplash API base URL.
const DefaultBaseURL = "https://api.unsplash.com"

const noKeyMsg = "Error: No Unsplash API key provided. Set UNSPLASH_ACCESS_KEY environment variable."

var (
	validOrientations = map[string]bool{"landscape": true, "portrait": true, "squarish": true}
	validColors       = map[string]bool{"black_and_white": true, "black": true, "white": true, "yellow": true, "orange": true,
		"red": true, "purple": true, "magenta": true, "green": true, "teal": true, "blue": true}
)

// UnsplashTools wraps the Unsplash API.
type UnsplashTools struct {
	toolkit.Toolkit
	accessKey  string
	baseURL    string
	httpClient *http.Client

	enableSearchPhotos   bool
	enableGetPhoto       bool
	enableGetRandomPhoto bool
	enableDownloadPhoto  bool
}

// Option configures UnsplashTools.
type Option func(*UnsplashTools)

// WithAccessKey sets the access key (default UNSPLASH_ACCESS_KEY).
func WithAccessKey(k string) Option { return func(t *UnsplashTools) { t.accessKey = k } }

// WithAPIKey is an alias of WithAccessKey.
func WithAPIKey(k string) Option { return WithAccessKey(k) }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *UnsplashTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *UnsplashTools) { t.httpClient = c } }

// WithTimeout sets the request timeout (default 30s).
func WithTimeout(d time.Duration) Option { return func(t *UnsplashTools) { t.Toolkit.Timeout = d } }

// WithSearchPhotos toggles SearchPhotos (default true).
func WithSearchPhotos(b bool) Option { return func(t *UnsplashTools) { t.enableSearchPhotos = b } }

// WithGetPhoto toggles GetPhoto (default true).
func WithGetPhoto(b bool) Option { return func(t *UnsplashTools) { t.enableGetPhoto = b } }

// WithGetRandomPhoto toggles GetRandomPhoto (default true).
func WithGetRandomPhoto(b bool) Option { return func(t *UnsplashTools) { t.enableGetRandomPhoto = b } }

// WithDownloadPhoto toggles DownloadPhoto (default false).
func WithDownloadPhoto(b bool) Option { return func(t *UnsplashTools) { t.enableDownloadPhoto = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(t *UnsplashTools) {
		t.enableSearchPhotos, t.enableGetPhoto, t.enableGetRandomPhoto, t.enableDownloadPhoto = true, true, true, true
	}
}

// SearchPhotosParams are the SearchPhotos parameters.
type SearchPhotosParams struct {
	Query       string `json:"query" description:"The search query string (e.g., \"mountain sunset\", \"office workspace\")." required:"true"`
	PerPage     int    `json:"per_page,omitempty" description:"Number of results per page (1-30). Default: 10."`
	Page        int    `json:"page,omitempty" description:"Page number to retrieve. Default: 1."`
	Orientation string `json:"orientation,omitempty" description:"Filter by orientation: \"landscape\", \"portrait\", or \"squarish\"."`
	Color       string `json:"color,omitempty" description:"Filter by color: black_and_white, black, white, yellow, orange, red, purple, magenta, green, teal, blue."`
}

// PhotoIDParams identifies a photo.
type PhotoIDParams struct {
	PhotoID string `json:"photo_id" description:"The unique identifier of the photo." required:"true"`
}

// RandomPhotoParams are the GetRandomPhoto parameters.
type RandomPhotoParams struct {
	Query       string `json:"query,omitempty" description:"Optional search query to filter random photos."`
	Orientation string `json:"orientation,omitempty" description:"Filter by orientation: \"landscape\", \"portrait\", or \"squarish\"."`
	Count       int    `json:"count,omitempty" description:"Number of random photos to return (1-30). Default: 1."`
}

// NewUnsplashTools creates the toolkit.
func NewUnsplashTools(opts ...Option) *UnsplashTools {
	t := &UnsplashTools{
		accessKey:            os.Getenv("UNSPLASH_ACCESS_KEY"),
		baseURL:              DefaultBaseURL,
		enableSearchPhotos:   true,
		enableGetPhoto:       true,
		enableGetRandomPhoto: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "unsplash_tools"
	t.Toolkit.Description = "Search and retrieve high-quality, royalty-free images from Unsplash."
	t.Toolkit.Timeout = 30 * time.Second
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableSearchPhotos {
		t.Toolkit.Register("SearchPhotos", "Search for photos on Unsplash by keyword. Returns JSON with photo details including URLs, author information, and metadata.", t, t.SearchPhotos, SearchPhotosParams{})
	}
	if t.enableGetPhoto {
		t.Toolkit.Register("GetPhoto", "Get detailed information about a specific photo, including URLs, author, metadata, EXIF data, and location if available.", t, t.GetPhoto, PhotoIDParams{})
	}
	if t.enableGetRandomPhoto {
		t.Toolkit.Register("GetRandomPhoto", "Get random photo(s) from Unsplash.", t, t.GetRandomPhoto, RandomPhotoParams{})
	}
	if t.enableDownloadPhoto {
		t.Toolkit.Register("DownloadPhoto", "Trigger a download event for a photo (required by the Unsplash API guidelines when a photo is used). Returns the download URL.", t, t.DownloadPhoto, PhotoIDParams{})
	}
	return t
}

func (t *UnsplashTools) request(endpoint string, q url.Values) ([]byte, error) {
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{
		"Authorization":  "Client-ID " + t.accessKey,
		"Accept-Version": "v1",
	})
	return c.Do(context.Background(), http.MethodGet, endpoint, q, nil)
}

type m = map[string]interface{}

func sub(v interface{}, key string) m {
	mm, _ := v.(m)
	r, _ := mm[key].(m)
	if r == nil {
		return m{}
	}
	return r
}

func formatPhoto(photo m) m {
	desc := photo["description"]
	if desc == nil || desc == "" {
		desc = photo["alt_description"]
	}
	urls := sub(photo, "urls")
	user := sub(photo, "user")
	links := sub(photo, "links")
	tags := []interface{}{}
	if list, ok := photo["tags"].([]interface{}); ok {
		for i, tg := range list {
			if i >= 5 {
				break
			}
			if title, _ := tg.(m)["title"].(string); title != "" {
				tags = append(tags, title)
			}
		}
	}
	return m{
		"id":          photo["id"],
		"description": desc,
		"width":       photo["width"],
		"height":      photo["height"],
		"color":       photo["color"],
		"created_at":  photo["created_at"],
		"urls": m{
			"raw": urls["raw"], "full": urls["full"], "regular": urls["regular"], "small": urls["small"], "thumb": urls["thumb"],
		},
		"author": m{
			"name": user["name"], "username": user["username"], "profile_url": sub(user, "links")["html"],
		},
		"links": m{"html": links["html"], "download": links["download"]},
		"likes": photo["likes"],
		"tags":  tags,
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// SearchPhotos searches photos by keyword.
func (t *UnsplashTools) SearchPhotos(p SearchPhotosParams) (interface{}, error) {
	if t.accessKey == "" {
		return noKeyMsg, nil
	}
	if p.Query == "" {
		return "Error: Please provide a search query.", nil
	}
	if p.PerPage == 0 {
		p.PerPage = 10
	}
	q := url.Values{
		"query":    {p.Query},
		"per_page": {strconv.Itoa(clamp(p.PerPage, 1, 30))},
		"page":     {strconv.Itoa(clamp(p.Page, 1, 1<<31-1))},
	}
	if validOrientations[p.Orientation] {
		q.Set("orientation", p.Orientation)
	}
	if validColors[p.Color] {
		q.Set("color", p.Color)
	}
	data, err := t.request("/search/photos", q)
	if err != nil {
		return fmt.Sprintf("Error searching Unsplash: %v", err), nil
	}
	var resp m
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Sprintf("Error searching Unsplash: %v", err), nil
	}
	photos := []m{}
	if list, ok := resp["results"].([]interface{}); ok {
		for _, ph := range list {
			if pm, ok := ph.(m); ok {
				photos = append(photos, formatPhoto(pm))
			}
		}
	}
	total, totalPages := resp["total"], resp["total_pages"]
	if total == nil {
		total = 0
	}
	if totalPages == nil {
		totalPages = 0
	}
	return restapi.ToJSON(m{"total": total, "total_pages": totalPages, "photos": photos}), nil
}

// GetPhoto returns details of a photo.
func (t *UnsplashTools) GetPhoto(p PhotoIDParams) (interface{}, error) {
	if t.accessKey == "" {
		return noKeyMsg, nil
	}
	if p.PhotoID == "" {
		return "Error: Please provide a photo ID.", nil
	}
	data, err := t.request("/photos/"+url.PathEscape(p.PhotoID), nil)
	if err != nil {
		return fmt.Sprintf("Error getting photo: %v", err), nil
	}
	var photo m
	if err := json.Unmarshal(data, &photo); err != nil {
		return fmt.Sprintf("Error getting photo: %v", err), nil
	}
	result := formatPhoto(photo)
	if exif, ok := photo["exif"].(m); ok && len(exif) > 0 {
		result["exif"] = m{
			"make": exif["make"], "model": exif["model"], "aperture": exif["aperture"],
			"exposure_time": exif["exposure_time"], "focal_length": exif["focal_length"], "iso": exif["iso"],
		}
	}
	if loc, ok := photo["location"].(m); ok && len(loc) > 0 {
		result["location"] = m{"name": loc["name"], "city": loc["city"], "country": loc["country"]}
	}
	result["views"] = photo["views"]
	result["downloads"] = photo["downloads"]
	return restapi.ToJSON(result), nil
}

// GetRandomPhoto returns random photo(s).
func (t *UnsplashTools) GetRandomPhoto(p RandomPhotoParams) (interface{}, error) {
	if t.accessKey == "" {
		return noKeyMsg, nil
	}
	if p.Count == 0 {
		p.Count = 1
	}
	q := url.Values{"count": {strconv.Itoa(clamp(p.Count, 1, 30))}}
	if p.Query != "" {
		q.Set("query", p.Query)
	}
	if validOrientations[p.Orientation] {
		q.Set("orientation", p.Orientation)
	}
	data, err := t.request("/photos/random", q)
	if err != nil {
		return fmt.Sprintf("Error getting random photo: %v", err), nil
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Sprintf("Error getting random photo: %v", err), nil
	}
	photos := []m{}
	switch v := raw.(type) {
	case []interface{}:
		for _, ph := range v {
			if pm, ok := ph.(m); ok {
				photos = append(photos, formatPhoto(pm))
			}
		}
	case m:
		photos = append(photos, formatPhoto(v))
	}
	return restapi.ToJSON(m{"photos": photos}), nil
}

// DownloadPhoto triggers a download event and returns the download URL.
func (t *UnsplashTools) DownloadPhoto(p PhotoIDParams) (interface{}, error) {
	if t.accessKey == "" {
		return noKeyMsg, nil
	}
	if p.PhotoID == "" {
		return "Error: Please provide a photo ID.", nil
	}
	data, err := t.request("/photos/"+url.PathEscape(p.PhotoID)+"/download", nil)
	if err != nil {
		return fmt.Sprintf("Error tracking download: %v", err), nil
	}
	var resp m
	_ = json.Unmarshal(data, &resp)
	return restapi.ToJSON(m{"photo_id": p.PhotoID, "download_url": resp["url"]}), nil
}

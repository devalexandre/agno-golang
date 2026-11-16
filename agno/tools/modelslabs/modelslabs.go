// Package modelslabs ports Agno's ModelsLabTools: video, image and audio
// generation through the ModelsLab REST API.
package modelslabs

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/mediaout"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the ModelsLab API root.
const DefaultBaseURL = "https://modelslab.com/api/v6"

// FileType selects what GenerateMedia produces.
type FileType string

// Supported file types (mirror agno.models.response.FileType).
const (
	FileTypeMP4 FileType = "mp4"
	FileTypeGIF FileType = "gif"
	FileTypeMP3 FileType = "mp3"
	FileTypeWAV FileType = "wav"
	FileTypePNG FileType = "png"
	FileTypeJPG FileType = "jpg"
)

var generatePaths = map[FileType]string{
	FileTypeMP4: "/video/text2video", FileTypeGIF: "/video/text2video",
	FileTypeMP3: "/voice/music_gen", FileTypeWAV: "/voice/sfx",
	FileTypePNG: "/images/text2img", FileTypeJPG: "/images/text2img",
}

var fetchPaths = map[FileType]string{
	FileTypeMP4: "/video/fetch", FileTypeGIF: "/video/fetch",
	FileTypeMP3: "/voice/fetch", FileTypeWAV: "/voice/fetch",
	FileTypePNG: "/images/fetch", FileTypeJPG: "/images/fetch",
}

// ModelsLabTools generates media with ModelsLab.
type ModelsLabTools struct {
	toolkit.Toolkit
	apiKey            string
	baseURL           string
	httpClient        *http.Client
	waitForCompletion bool
	addToETA          int
	maxWaitTime       int
	fileType          FileType
	modelID           string
	width, height     int
	pollInterval      time.Duration
}

// Option configures ModelsLabTools.
type Option func(*ModelsLabTools)

// WithAPIKey sets the API key (default MODELS_LAB_API_KEY).
func WithAPIKey(k string) Option { return func(t *ModelsLabTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *ModelsLabTools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *ModelsLabTools) { t.httpClient = c } }

// WithTimeout sets the request timeout (default 120s).
func WithTimeout(d time.Duration) Option { return func(t *ModelsLabTools) { t.Toolkit.Timeout = d } }

// WithWaitForCompletion polls until the media is ready (default false).
func WithWaitForCompletion(on bool) Option {
	return func(t *ModelsLabTools) { t.waitForCompletion = on }
}

// WithAddToETA adds seconds to the reported ETA when waiting (default 15).
func WithAddToETA(s int) Option { return func(t *ModelsLabTools) { t.addToETA = s } }

// WithMaxWaitTime caps the wait in seconds (default 60).
func WithMaxWaitTime(s int) Option { return func(t *ModelsLabTools) { t.maxWaitTime = s } }

// WithFileType selects the output type (default mp4).
func WithFileType(f FileType) Option { return func(t *ModelsLabTools) { t.fileType = f } }

// WithModelID overrides the model id.
func WithModelID(id string) Option { return func(t *ModelsLabTools) { t.modelID = id } }

// WithSize sets width and height (default 512x512).
func WithSize(w, h int) Option { return func(t *ModelsLabTools) { t.width, t.height = w, h } }

// WithPollInterval sets the wait between fetch attempts (default 1s).
func WithPollInterval(d time.Duration) Option { return func(t *ModelsLabTools) { t.pollInterval = d } }

// GenerateMediaParams are the parameters of GenerateMedia.
type GenerateMediaParams struct {
	Prompt string `json:"prompt" description:"The prompt describing the media to generate." required:"true"`
}

// NewModelsLabTools creates the toolkit.
func NewModelsLabTools(opts ...Option) *ModelsLabTools {
	t := &ModelsLabTools{
		apiKey: os.Getenv("MODELS_LAB_API_KEY"), baseURL: DefaultBaseURL,
		addToETA: 15, maxWaitTime: 60, fileType: FileTypeMP4, width: 512, height: 512,
		pollInterval: time.Second,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "models_labs"
	t.Toolkit.Description = "Generate media (video, image, or audio) with ModelsLab."
	t.Toolkit.Timeout = 120 * time.Second
	for _, o := range opts {
		o(t)
	}
	if _, ok := generatePaths[t.fileType]; !ok {
		t.fileType = FileTypeMP4
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	t.Toolkit.Register("GenerateMedia", "Generate media (video, image, or audio) given a prompt.", t, t.GenerateMedia, GenerateMediaParams{})
	return t
}

func (t *ModelsLabTools) payload(prompt string) map[string]interface{} {
	p := map[string]interface{}{"key": t.apiKey, "prompt": prompt, "webhook": nil, "track_id": nil}
	model := func(def string) string {
		if t.modelID != "" {
			return t.modelID
		}
		return def
	}
	switch t.fileType {
	case FileTypePNG, FileTypeJPG:
		p["model_id"] = model("flux")
		p["width"], p["height"] = t.width, t.height
		p["samples"], p["num_inference_steps"], p["safety_checker"] = 1, 30, "no"
	case FileTypeMP4, FileTypeGIF:
		p["height"], p["width"] = t.height, t.width
		p["num_frames"], p["negative_prompt"] = 25, "low quality"
		p["model_id"] = model("cogvideox")
		p["instant_response"], p["output_type"] = false, string(t.fileType)
	case FileTypeWAV:
		p["duration"], p["output_format"], p["temp"] = 10, "wav", false
	default:
		p["base64"], p["temp"] = false, false
	}
	return p
}

func (t *ModelsLabTools) waitForMedia(c *restapi.Client, mediaID string, eta int) bool {
	wait := eta + t.addToETA
	if wait > t.maxWaitTime {
		wait = t.maxWaitTime
	}
	deadline := time.Now().Add(time.Duration(wait) * time.Second)
	for i := 0; i < wait && time.Now().Before(deadline); i++ {
		var res map[string]interface{}
		if err := c.JSON(context.Background(), http.MethodPost, fetchPaths[t.fileType]+"/"+mediaID, nil,
			map[string]string{"key": t.apiKey}, &res); err == nil && res["status"] == "success" {
			return true
		}
		time.Sleep(t.pollInterval)
	}
	return false
}

// GenerateMedia generates media (video, image, or audio) given a prompt.
func (t *ModelsLabTools) GenerateMedia(p GenerateMediaParams) (interface{}, error) {
	if t.apiKey == "" {
		return "Please set the MODELS_LAB_API_KEY", nil
	}
	c := restapi.New(t.baseURL, t.httpClient, nil)
	var result map[string]interface{}
	if err := c.JSON(context.Background(), http.MethodPost, generatePaths[t.fileType], nil, t.payload(p.Prompt), &result); err != nil {
		return fmt.Sprintf("Error: Network error while generating %s: %v", t.fileType, err), nil
	}
	if result["status"] == "error" {
		return fmt.Sprintf("Error: %v", result["message"]), nil
	}
	if e, ok := result["error"]; ok {
		return fmt.Sprintf("Error: %v", e), nil
	}
	eta, hasETA := result["eta"]
	etaStr := ""
	if hasETA && eta != nil {
		etaStr = fmt.Sprint(eta)
	}
	mediaID := mediaout.NewID()

	links := func(keys ...string) []string {
		for _, k := range keys {
			if arr, ok := result[k].([]interface{}); ok && len(arr) > 0 {
				out := make([]string, 0, len(arr))
				for _, v := range arr {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
		}
		return nil
	}
	var urls []string
	switch t.fileType {
	case FileTypePNG, FileTypeJPG, FileTypeWAV:
		urls = links("output", "future_links")
	default:
		urls = links("future_links", "output")
	}

	res := mediaout.Result{}
	for _, u := range urls {
		a := mediaout.Artifact{ID: mediaID, URL: u}
		switch t.fileType {
		case FileTypePNG, FileTypeJPG, FileTypeGIF:
			res.Images = append(res.Images, a)
		case FileTypeMP4:
			a.ETA = fmt.Sprint(eta)
			res.Videos = append(res.Videos, a)
		default:
			res.Audios = append(res.Audios, a)
		}
		if etaNum, ok := eta.(float64); ok && t.waitForCompletion && etaNum == float64(int(etaNum)) {
			t.waitForMedia(c, mediaID, int(etaNum))
		}
	}

	kind := string(t.fileType)
	capitalized := strings.ToUpper(kind[:1]) + kind[1:]
	switch {
	case (t.fileType == FileTypePNG || t.fileType == FileTypeJPG) && etaStr == "":
		res.Content = fmt.Sprintf("Image generated successfully (%s)", strings.ToUpper(kind))
	case etaStr != "":
		res.Content = fmt.Sprintf("%s has been generated successfully and will be ready in %s seconds", capitalized, etaStr)
	default:
		res.Content = fmt.Sprintf("%s generated successfully", capitalized)
	}
	return res.String(), nil
}

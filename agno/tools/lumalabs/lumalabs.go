// Package lumalabs ports Agno's LumaLabTools: text-to-video and image-to-video
// generation with the Luma Dream Machine REST API.
package lumalabs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultBaseURL is the Luma Dream Machine API base URL.
const DefaultBaseURL = "https://api.lumalabs.ai/dream-machine/v1"

// Video is a generated video artifact.
type Video struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	State string `json:"state"`
}

// LumaLabTools wraps the Luma API.
type LumaLabTools struct {
	toolkit.Toolkit
	apiKey            string
	baseURL           string
	waitForCompletion bool
	pollInterval      time.Duration
	maxWaitTime       time.Duration
	httpClient        *http.Client

	enableGenerateVideo bool
	enableImageToVideo  bool

	mu     sync.Mutex
	videos []Video
}

// Option configures LumaLabTools.
type Option func(*LumaLabTools)

// WithAPIKey sets the API key (default LUMAAI_API_KEY).
func WithAPIKey(k string) Option { return func(t *LumaLabTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *LumaLabTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *LumaLabTools) { t.httpClient = c } }

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option { return func(t *LumaLabTools) { t.Toolkit.Timeout = d } }

// WithWaitForCompletion toggles waiting for the generation (default true).
func WithWaitForCompletion(b bool) Option { return func(t *LumaLabTools) { t.waitForCompletion = b } }

// WithPollInterval sets the polling interval (default 3s).
func WithPollInterval(d time.Duration) Option { return func(t *LumaLabTools) { t.pollInterval = d } }

// WithMaxWaitTime sets the maximum wait (default 5 minutes).
func WithMaxWaitTime(d time.Duration) Option { return func(t *LumaLabTools) { t.maxWaitTime = d } }

// WithGenerateVideo toggles GenerateVideo (default true).
func WithGenerateVideo(b bool) Option { return func(t *LumaLabTools) { t.enableGenerateVideo = b } }

// WithImageToVideo toggles ImageToVideo (default true).
func WithImageToVideo(b bool) Option { return func(t *LumaLabTools) { t.enableImageToVideo = b } }

// ImageToVideoParams are the ImageToVideo parameters.
type ImageToVideoParams struct {
	Prompt        string `json:"prompt" description:"Text description of the desired video" required:"true"`
	StartImageURL string `json:"start_image_url" description:"URL of the starting image" required:"true"`
	EndImageURL   string `json:"end_image_url,omitempty" description:"Optional URL of the ending image"`
	Loop          bool   `json:"loop,omitempty" description:"Whether the video should loop"`
	AspectRatio   string `json:"aspect_ratio,omitempty" description:"Aspect ratio of the output video: 1:1, 16:9, 9:16, 4:3, 3:4, 21:9, 9:21. Default 16:9."`
}

// GenerateVideoParams are the GenerateVideo parameters.
type GenerateVideoParams struct {
	Prompt      string                       `json:"prompt" description:"Text description of the desired video" required:"true"`
	Loop        bool                         `json:"loop,omitempty" description:"Whether the video should loop"`
	AspectRatio string                       `json:"aspect_ratio,omitempty" description:"Aspect ratio of the output video: 1:1, 16:9, 9:16, 4:3, 3:4, 21:9, 9:21. Default 16:9."`
	Keyframes   map[string]map[string]string `json:"keyframes,omitempty" description:"Optional keyframes, e.g. {\"frame0\": {\"type\": \"image\", \"url\": \"...\"}}"`
}

// NewLumaLabTools creates the toolkit.
func NewLumaLabTools(opts ...Option) *LumaLabTools {
	t := &LumaLabTools{
		apiKey:              os.Getenv("LUMAAI_API_KEY"),
		baseURL:             DefaultBaseURL,
		waitForCompletion:   true,
		pollInterval:        3 * time.Second,
		maxWaitTime:         5 * time.Minute,
		enableGenerateVideo: true,
		enableImageToVideo:  true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "luma_lab"
	t.Toolkit.Description = "Generate videos from text or images with Luma AI Dream Machine."
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableGenerateVideo {
		t.Toolkit.Register("GenerateVideo", "Use this function to generate a video given a prompt.", t, t.GenerateVideo, GenerateVideoParams{})
	}
	if t.enableImageToVideo {
		t.Toolkit.Register("ImageToVideo", "Generate a video from one or two images with a prompt.", t, t.ImageToVideo, ImageToVideoParams{})
	}
	return t
}

// Videos returns the video artifacts generated so far.
func (t *LumaLabTools) Videos() []Video {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Video(nil), t.videos...)
}

// ImageToVideo generates a video from start (and optional end) keyframe images.
func (t *LumaLabTools) ImageToVideo(p ImageToVideoParams) (interface{}, error) {
	keyframes := map[string]map[string]string{"frame0": {"type": "image", "url": p.StartImageURL}}
	if p.EndImageURL != "" {
		keyframes["frame1"] = map[string]string{"type": "image", "url": p.EndImageURL}
	}
	return t.generate(p.Prompt, p.Loop, p.AspectRatio, keyframes), nil
}

// GenerateVideo generates a video from a prompt.
func (t *LumaLabTools) GenerateVideo(p GenerateVideoParams) (interface{}, error) {
	return t.generate(p.Prompt, p.Loop, p.AspectRatio, p.Keyframes), nil
}

type generation struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	FailureReason string `json:"failure_reason"`
	Assets        *struct {
		Video string `json:"video"`
	} `json:"assets"`
}

func (t *LumaLabTools) generate(prompt string, loop bool, aspectRatio string, keyframes map[string]map[string]string) string {
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	body := map[string]interface{}{"prompt": prompt, "loop": loop, "aspect_ratio": aspectRatio}
	if keyframes != nil {
		body["keyframes"] = keyframes
	}
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Bearer " + t.apiKey})
	ctx := context.Background()
	var gen generation
	if err := c.JSON(ctx, http.MethodPost, "/generations", nil, body, &gen); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if !t.waitForCompletion {
		return "Async generation unsupported"
	}
	var waited time.Duration
	for waited < t.maxWaitTime {
		if gen.ID == "" {
			return "Failed to get generation ID"
		}
		if err := c.JSON(ctx, http.MethodGet, "/generations/"+url.PathEscape(gen.ID), nil, nil, &gen); err != nil {
			return fmt.Sprintf("Error: %v", err)
		}
		if gen.State == "completed" && gen.Assets != nil && gen.Assets.Video != "" {
			t.mu.Lock()
			t.videos = append(t.videos, Video{ID: uuid.NewString(), URL: gen.Assets.Video, State: "completed"})
			t.mu.Unlock()
			return fmt.Sprintf("Video generated successfully: %s", gen.Assets.Video)
		}
		if gen.State == "failed" {
			return fmt.Sprintf("Generation failed: %s", gen.FailureReason)
		}
		time.Sleep(t.pollInterval)
		waited += t.pollInterval
	}
	return fmt.Sprintf("Video generation timed out after %d seconds", int(t.maxWaitTime.Seconds()))
}

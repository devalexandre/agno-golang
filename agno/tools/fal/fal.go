// Package fal ports Agno's FalTools: image/video generation with fal.ai models
// through the fal queue REST API.
package fal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultQueueURL is the fal queue API base URL.
const DefaultQueueURL = "https://queue.fal.run"

// ImageToImageModel is the model used by ImageToImage.
const ImageToImageModel = "fal-ai/flux/dev/image-to-image"

// Media is a generated image or video artifact.
type Media struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // "image" or "video"
	URL  string `json:"url"`
}

// FalTools wraps fal.ai.
type FalTools struct {
	toolkit.Toolkit
	apiKey       string
	baseURL      string
	model        string
	pollInterval time.Duration
	maxWait      time.Duration
	httpClient   *http.Client

	enableGenerateMedia bool
	enableImageToImage  bool

	mu    sync.Mutex
	media []Media
}

// Option configures FalTools.
type Option func(*FalTools)

// WithAPIKey sets the API key (default FAL_API_KEY, then FAL_KEY).
func WithAPIKey(k string) Option { return func(t *FalTools) { t.apiKey = k } }

// WithBaseURL overrides the queue API base URL.
func WithBaseURL(u string) Option { return func(t *FalTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *FalTools) { t.httpClient = c } }

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option { return func(t *FalTools) { t.Toolkit.Timeout = d } }

// WithModel sets the model used by GenerateMedia (default fal-ai/hunyuan-video).
func WithModel(m string) Option { return func(t *FalTools) { t.model = m } }

// WithPollInterval sets the queue polling interval (default 1s).
func WithPollInterval(d time.Duration) Option { return func(t *FalTools) { t.pollInterval = d } }

// WithMaxWait bounds how long a job is awaited (default 10 minutes).
func WithMaxWait(d time.Duration) Option { return func(t *FalTools) { t.maxWait = d } }

// WithGenerateMedia toggles GenerateMedia (default true).
func WithGenerateMedia(b bool) Option { return func(t *FalTools) { t.enableGenerateMedia = b } }

// WithImageToImage toggles ImageToImage (default false).
func WithImageToImage(b bool) Option { return func(t *FalTools) { t.enableImageToImage = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(t *FalTools) { t.enableGenerateMedia, t.enableImageToImage = true, true }
}

// GenerateMediaParams are the GenerateMedia parameters.
type GenerateMediaParams struct {
	Prompt string `json:"prompt" description:"A text description of the task." required:"true"`
}

// ImageToImageParams are the ImageToImage parameters.
type ImageToImageParams struct {
	Prompt   string `json:"prompt" description:"A text description of the task." required:"true"`
	ImageURL string `json:"image_url,omitempty" description:"The URL of the image to use for the generation."`
}

// NewFalTools creates the toolkit.
func NewFalTools(opts ...Option) *FalTools {
	key := os.Getenv("FAL_API_KEY")
	if key == "" {
		key = os.Getenv("FAL_KEY")
	}
	t := &FalTools{
		apiKey:              key,
		baseURL:             DefaultQueueURL,
		model:               "fal-ai/hunyuan-video",
		pollInterval:        time.Second,
		maxWait:             10 * time.Minute,
		enableGenerateMedia: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "fal-tools"
	t.Toolkit.Description = "Generate images and videos with fal.ai models."
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableGenerateMedia {
		t.Toolkit.Register("GenerateMedia", "Use this function to run a model with a given prompt.", t, t.GenerateMedia, GenerateMediaParams{})
	}
	if t.enableImageToImage {
		t.Toolkit.Register("ImageToImage", "Use this function to transform an input image based on a text prompt using the Fal AI image-to-image model. The model takes an existing image and generates a new version modified according to your prompt.", t, t.ImageToImage, ImageToImageParams{})
	}
	return t
}

// Media returns the media artifacts generated so far.
func (t *FalTools) Media() []Media {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Media(nil), t.media...)
}

func (t *FalTools) add(kind, url string) {
	t.mu.Lock()
	t.media = append(t.media, Media{ID: uuid.NewString(), Kind: kind, URL: url})
	t.mu.Unlock()
}

// subscribe submits a job to the fal queue and waits for its result.
func (t *FalTools) subscribe(model string, args map[string]interface{}) (map[string]interface{}, error) {
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Key " + t.apiKey})
	ctx := context.Background()
	var sub struct {
		RequestID   string `json:"request_id"`
		StatusURL   string `json:"status_url"`
		ResponseURL string `json:"response_url"`
	}
	if err := c.JSON(ctx, http.MethodPost, "/"+strings.TrimLeft(model, "/"), nil, args, &sub); err != nil {
		return nil, err
	}
	if sub.RequestID == "" && sub.StatusURL == "" {
		return nil, fmt.Errorf("fal queue did not return a request id")
	}
	appID := appID(model)
	if sub.StatusURL == "" {
		sub.StatusURL = fmt.Sprintf("/%s/requests/%s/status", appID, sub.RequestID)
	}
	if sub.ResponseURL == "" {
		sub.ResponseURL = fmt.Sprintf("/%s/requests/%s", appID, sub.RequestID)
	}
	deadline := time.Now().Add(t.maxWait)
	for {
		var st struct {
			Status string `json:"status"`
		}
		if err := c.JSON(ctx, http.MethodGet, sub.StatusURL, nil, nil, &st); err != nil {
			return nil, err
		}
		if st.Status == "COMPLETED" {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("fal job timed out after %s (status %s)", t.maxWait, st.Status)
		}
		time.Sleep(t.pollInterval)
	}
	var result map[string]interface{}
	if err := c.JSON(ctx, http.MethodGet, sub.ResponseURL, nil, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// appID returns owner/name of a model id (fal status URLs drop sub-paths).
func appID(model string) string {
	parts := strings.Split(strings.Trim(model, "/"), "/")
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, "/")
}

// GenerateMedia runs the configured model with the prompt.
func (t *FalTools) GenerateMedia(p GenerateMediaParams) (interface{}, error) {
	result, err := t.subscribe(t.model, map[string]interface{}{"prompt": p.Prompt})
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	if urls := listURLs(result["images"]); len(urls) > 0 {
		for _, u := range urls {
			t.add("image", u)
		}
		return fmt.Sprintf("Generated %d image(s) successfully: %s", len(urls), strings.Join(urls, ", ")), nil
	}
	if img, ok := result["image"].(map[string]interface{}); ok {
		u, _ := img["url"].(string)
		t.add("image", u)
		return fmt.Sprintf("Image generated successfully at %s", u), nil
	}
	if urls := listURLs(result["videos"]); len(urls) > 0 {
		for _, u := range urls {
			t.add("video", u)
		}
		return fmt.Sprintf("Generated %d video(s) successfully: %s", len(urls), strings.Join(urls, ", ")), nil
	}
	if vid, ok := result["video"].(map[string]interface{}); ok {
		u, _ := vid["url"].(string)
		t.add("video", u)
		return fmt.Sprintf("Video generated successfully at %s", u), nil
	}
	raw, _ := json.Marshal(result)
	return fmt.Sprintf("Unsupported type in result: %s", raw), nil
}

// ImageToImage transforms an image based on a prompt.
func (t *FalTools) ImageToImage(p ImageToImageParams) (interface{}, error) {
	result, err := t.subscribe(ImageToImageModel, map[string]interface{}{"image_url": p.ImageURL, "prompt": p.Prompt})
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	u := ""
	if urls := listURLs(result["images"]); len(urls) > 0 {
		u = urls[0]
	}
	t.add("image", u)
	return fmt.Sprintf("Image generated successfully at %s", u), nil
}

func listURLs(v interface{}) []string {
	list, _ := v.([]interface{})
	var out []string
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			if u, _ := m["url"].(string); u != "" {
				out = append(out, u)
			}
		}
	}
	return out
}

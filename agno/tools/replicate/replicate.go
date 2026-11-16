// Package replicate ports Agno's ReplicateTools: image/video generation with
// Replicate models through the Replicate predictions REST API.
package replicate

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultBaseURL is the Replicate API base URL.
const DefaultBaseURL = "https://api.replicate.com"

var (
	imageExtensions = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".tiff": true, ".webp": true}
	videoExtensions = map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".flv": true, ".wmv": true, ".webm": true}
)

// Media is a generated image or video artifact.
type Media struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// ReplicateTools wraps the Replicate API.
type ReplicateTools struct {
	toolkit.Toolkit
	apiKey       string
	baseURL      string
	model        string
	pollInterval time.Duration
	maxWait      time.Duration
	httpClient   *http.Client

	enableGenerateMedia bool

	mu    sync.Mutex
	media []Media
}

// Option configures ReplicateTools.
type Option func(*ReplicateTools)

// WithAPIKey sets the API token (default REPLICATE_API_KEY, then REPLICATE_API_TOKEN).
func WithAPIKey(k string) Option { return func(t *ReplicateTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *ReplicateTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *ReplicateTools) { t.httpClient = c } }

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option { return func(t *ReplicateTools) { t.Toolkit.Timeout = d } }

// WithModel sets the model ("owner/name" or "owner/name:version"; default minimax/video-01).
func WithModel(m string) Option { return func(t *ReplicateTools) { t.model = m } }

// WithPollInterval sets the prediction polling interval (default 1s).
func WithPollInterval(d time.Duration) Option { return func(t *ReplicateTools) { t.pollInterval = d } }

// WithMaxWait bounds how long a prediction is awaited (default 10 minutes).
func WithMaxWait(d time.Duration) Option { return func(t *ReplicateTools) { t.maxWait = d } }

// WithGenerateMedia toggles GenerateMedia (default true).
func WithGenerateMedia(b bool) Option { return func(t *ReplicateTools) { t.enableGenerateMedia = b } }

// GenerateMediaParams are the GenerateMedia parameters.
type GenerateMediaParams struct {
	Prompt string `json:"prompt" description:"A text description of the content." required:"true"`
}

// NewReplicateTools creates the toolkit.
func NewReplicateTools(opts ...Option) *ReplicateTools {
	key := os.Getenv("REPLICATE_API_KEY")
	if key == "" {
		key = os.Getenv("REPLICATE_API_TOKEN")
	}
	t := &ReplicateTools{
		apiKey:              key,
		baseURL:             DefaultBaseURL,
		model:               "minimax/video-01",
		pollInterval:        time.Second,
		maxWait:             10 * time.Minute,
		enableGenerateMedia: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "replicate_toolkit"
	t.Toolkit.Description = "Generate images or videos with Replicate models."
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableGenerateMedia {
		t.Toolkit.Register("GenerateMedia", "Use this function to generate an image or a video using a replicate model.", t, t.GenerateMedia, GenerateMediaParams{})
	}
	return t
}

// Media returns the media artifacts generated so far.
func (t *ReplicateTools) Media() []Media {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Media(nil), t.media...)
}

type prediction struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Output interface{} `json:"output"`
	Error  interface{} `json:"error"`
}

// GenerateMedia runs the model with the prompt and returns the media URLs.
func (t *ReplicateTools) GenerateMedia(p GenerateMediaParams) (interface{}, error) {
	if t.apiKey == "" {
		return "API key is not set.", nil
	}
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Bearer " + t.apiKey, "Prefer": "wait"})
	ctx := context.Background()
	input := map[string]interface{}{"prompt": p.Prompt}

	var pred prediction
	var err error
	if _, version, ok := strings.Cut(t.model, ":"); ok && version != "" {
		err = c.JSON(ctx, http.MethodPost, "/v1/predictions", nil, map[string]interface{}{"version": version, "input": input}, &pred)
	} else {
		err = c.JSON(ctx, http.MethodPost, "/v1/models/"+strings.Trim(t.model, "/")+"/predictions", nil, map[string]interface{}{"input": input}, &pred)
	}
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	deadline := time.Now().Add(t.maxWait)
	for pred.Status != "succeeded" {
		switch pred.Status {
		case "failed", "canceled":
			return fmt.Sprintf("Error: prediction %s: %v", pred.Status, pred.Error), nil
		}
		if time.Now().After(deadline) {
			return fmt.Sprintf("Error: prediction timed out after %s", t.maxWait), nil
		}
		time.Sleep(t.pollInterval)
		if err := c.JSON(ctx, http.MethodGet, "/v1/predictions/"+url.PathEscape(pred.ID), nil, nil, &pred); err != nil {
			return fmt.Sprintf("Error: %v", err), nil
		}
	}

	var outputs []string
	switch o := pred.Output.(type) {
	case string:
		outputs = []string{o}
	case []interface{}:
		for _, v := range o {
			s, ok := v.(string)
			if !ok {
				return fmt.Sprintf("Unexpected output type: %T", v), nil
			}
			outputs = append(outputs, s)
		}
	default:
		return fmt.Sprintf("Unexpected output type: %T", pred.Output), nil
	}

	var results []string
	var media []Media
	for _, out := range outputs {
		u, err := url.Parse(out)
		if err != nil {
			return fmt.Sprintf("Error: %v", err), nil
		}
		ext := strings.ToLower(path.Ext(u.Path))
		var kind string
		switch {
		case imageExtensions[ext]:
			kind = "image"
		case videoExtensions[ext]:
			kind = "video"
		default:
			return fmt.Sprintf("Error: Unsupported media type with extension '%s'.", ext), nil
		}
		media = append(media, Media{ID: uuid.NewString(), Kind: kind, URL: out})
		results = append(results, fmt.Sprintf("%s generated successfully at %s", strings.ToUpper(kind[:1])+kind[1:], out))
	}
	t.mu.Lock()
	t.media = append(t.media, media...)
	t.mu.Unlock()
	return strings.Join(results, "\n"), nil
}

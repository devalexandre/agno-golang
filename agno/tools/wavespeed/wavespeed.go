// Package wavespeed ports Agno's WaveSpeedTools: image and video generation
// through the WaveSpeed AI REST API (v3 predictions).
package wavespeed

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

// DefaultBaseURL is the WaveSpeed API root.
const DefaultBaseURL = "https://api.wavespeed.ai/api/v3"

// WaveSpeedTools generates images and videos with WaveSpeed models.
type WaveSpeedTools struct {
	toolkit.Toolkit
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	imageModel   string
	videoModel   string
	pollInterval time.Duration
	jobTimeout   time.Duration

	enableImage bool
	enableVideo bool
}

// Option configures WaveSpeedTools.
type Option func(*WaveSpeedTools)

// WithAPIKey sets the API key (default WAVESPEED_API_KEY).
func WithAPIKey(k string) Option { return func(t *WaveSpeedTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *WaveSpeedTools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *WaveSpeedTools) { t.httpClient = c } }

// WithTimeout sets the overall wait for a prediction (default 600s).
func WithTimeout(d time.Duration) Option { return func(t *WaveSpeedTools) { t.jobTimeout = d } }

// WithPollInterval sets the polling interval (default 1s).
func WithPollInterval(d time.Duration) Option { return func(t *WaveSpeedTools) { t.pollInterval = d } }

// WithImageModel sets the default image model.
func WithImageModel(m string) Option { return func(t *WaveSpeedTools) { t.imageModel = m } }

// WithVideoModel sets the default video model.
func WithVideoModel(m string) Option { return func(t *WaveSpeedTools) { t.videoModel = m } }

// WithGenerateImage enables/disables GenerateImage (default true).
func WithGenerateImage(on bool) Option { return func(t *WaveSpeedTools) { t.enableImage = on } }

// WithGenerateVideo enables/disables GenerateVideo (default true).
func WithGenerateVideo(on bool) Option { return func(t *WaveSpeedTools) { t.enableVideo = on } }

// WithAll enables every tool.
func WithAll() Option { return func(t *WaveSpeedTools) { t.enableImage, t.enableVideo = true, true } }

// GenerateParams are the parameters of GenerateImage and GenerateVideo.
type GenerateParams struct {
	Prompt string `json:"prompt" description:"A text description of the media to generate." required:"true"`
	Model  string `json:"model,omitempty" description:"Optional WaveSpeed model id to use instead of the default model."`
}

// NewWaveSpeedTools creates the toolkit.
func NewWaveSpeedTools(opts ...Option) *WaveSpeedTools {
	t := &WaveSpeedTools{
		apiKey: os.Getenv("WAVESPEED_API_KEY"), baseURL: DefaultBaseURL,
		imageModel: "bytedance/seedream-v5.0-pro", videoModel: "bytedance/seedance-2.5/text-to-video",
		pollInterval: time.Second, jobTimeout: 600 * time.Second,
		enableImage: true, enableVideo: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "wavespeed_tools"
	t.Toolkit.Description = "Generate images and videos with WaveSpeed AI models."
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableImage {
		t.Toolkit.Register("GenerateImage", "Use this function to generate an image from a text prompt using a WaveSpeed model. See https://wavespeed.ai/models for the full model catalog.", t, t.GenerateImage, GenerateParams{})
	}
	if t.enableVideo {
		t.Toolkit.Register("GenerateVideo", "Use this function to generate a video from a text prompt using a WaveSpeed model. See https://wavespeed.ai/models for the full model catalog.", t, t.GenerateVideo, GenerateParams{})
	}
	return t
}

type prediction struct {
	Data struct {
		ID      string   `json:"id"`
		Status  string   `json:"status"`
		Outputs []string `json:"outputs"`
		Error   string   `json:"error"`
	} `json:"data"`
}

// runModel submits a prediction and waits for its output URLs.
func (t *WaveSpeedTools) runModel(model string, args map[string]interface{}) ([]string, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("WAVESPEED_API_KEY not set")
	}
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Bearer " + t.apiKey})
	ctx := context.Background()
	var p prediction
	if err := c.JSON(ctx, http.MethodPost, "/"+strings.TrimLeft(model, "/"), nil, args, &p); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(t.jobTimeout)
	for {
		switch p.Data.Status {
		case "completed":
			return p.Data.Outputs, nil
		case "failed":
			return nil, fmt.Errorf("prediction failed: %s", p.Data.Error)
		}
		if p.Data.ID == "" {
			return p.Data.Outputs, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("prediction %s timed out after %s", p.Data.ID, t.jobTimeout)
		}
		time.Sleep(t.pollInterval)
		id := p.Data.ID
		p = prediction{}
		if err := c.JSON(ctx, http.MethodGet, "/predictions/"+id+"/result", nil, nil, &p); err != nil {
			return nil, err
		}
		if p.Data.ID == "" {
			p.Data.ID = id
		}
	}
}

func (t *WaveSpeedTools) generate(kind, model, prompt string) string {
	outputs, err := t.runModel(model, map[string]interface{}{"prompt": prompt})
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if len(outputs) == 0 {
		return "No output received from the model."
	}
	res := mediaout.Result{Content: fmt.Sprintf("Generated %d %s(s) successfully: %s", len(outputs), kind, strings.Join(outputs, ", "))}
	for _, u := range outputs {
		a := mediaout.Artifact{ID: mediaout.NewID(), URL: u}
		if kind == "image" {
			res.Images = append(res.Images, a)
		} else {
			res.Videos = append(res.Videos, a)
		}
	}
	return res.String()
}

// GenerateImage generates an image from a text prompt.
func (t *WaveSpeedTools) GenerateImage(p GenerateParams) (interface{}, error) {
	model := p.Model
	if model == "" {
		model = t.imageModel
	}
	return t.generate("image", model, p.Prompt), nil
}

// GenerateVideo generates a video from a text prompt.
func (t *WaveSpeedTools) GenerateVideo(p GenerateParams) (interface{}, error) {
	model := p.Model
	if model == "" {
		model = t.videoModel
	}
	return t.generate("video", model, p.Prompt), nil
}

// Package minimax ports Agno's MiniMaxTools: text-to-video generation through
// the MiniMax video generation REST API.
package minimax

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

// VideoURLs maps a region to its video generation endpoint.
var VideoURLs = map[string]string{
	"global_en": "https://api.minimax.io/v2/video_generation",
	"cn_zh":     "https://api.minimaxi.com/v2/video_generation",
}

// minPollRequest is the minimum budget needed to start another poll request.
const minPollRequest = time.Second

// MiniMaxTools generates videos with MiniMax.
type MiniMaxTools struct {
	toolkit.Toolkit
	apiKey       string
	baseURL      string
	region       string
	model        string
	pollInterval time.Duration
	maxWaitTime  time.Duration
	httpClient   *http.Client
	enableVideo  bool
}

// Option configures MiniMaxTools.
type Option func(*MiniMaxTools)

// WithAPIKey sets the API key (default MINIMAX_API_KEY).
func WithAPIKey(k string) Option { return func(t *MiniMaxTools) { t.apiKey = k } }

// WithBaseURL overrides the video generation endpoint.
func WithBaseURL(u string) Option { return func(t *MiniMaxTools) { t.baseURL = u } }

// WithRegion selects global_en (default) or cn_zh.
func WithRegion(r string) Option { return func(t *MiniMaxTools) { t.region = r } }

// WithModel sets the model (default MiniMax-H3).
func WithModel(m string) Option { return func(t *MiniMaxTools) { t.model = m } }

// WithPollInterval sets the polling interval (default 5s).
func WithPollInterval(d time.Duration) Option { return func(t *MiniMaxTools) { t.pollInterval = d } }

// WithMaxWaitTime caps the total wait (default 600s).
func WithMaxWaitTime(d time.Duration) Option { return func(t *MiniMaxTools) { t.maxWaitTime = d } }

// WithTimeout sets the per-request timeout (default 30s).
func WithTimeout(d time.Duration) Option { return func(t *MiniMaxTools) { t.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *MiniMaxTools) { t.httpClient = c } }

// WithGenerateVideo enables/disables GenerateVideo (default true).
func WithGenerateVideo(on bool) Option { return func(t *MiniMaxTools) { t.enableVideo = on } }

// WithAll enables every tool.
func WithAll() Option { return func(t *MiniMaxTools) { t.enableVideo = true } }

// GenerateVideoParams are the parameters of GenerateVideo.
type GenerateVideoParams struct {
	Prompt     string `json:"prompt" description:"Text description of the video to generate." required:"true"`
	Resolution string `json:"resolution,omitempty" description:"Output resolution. MiniMax H3 supports 768P or 2K. Default 2K."`
	Duration   int    `json:"duration,omitempty" description:"Video duration in seconds, from 4 through 15. Default 5."`
	Ratio      string `json:"ratio,omitempty" description:"Output aspect ratio, such as 16:9 or 9:16. Default 16:9."`
}

// NewMiniMaxTools creates the toolkit. An unknown region falls back to global_en.
func NewMiniMaxTools(opts ...Option) *MiniMaxTools {
	t := &MiniMaxTools{
		apiKey: os.Getenv("MINIMAX_API_KEY"), region: "global_en", model: "MiniMax-H3",
		pollInterval: 5 * time.Second, maxWaitTime: 600 * time.Second, enableVideo: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "minimax_tools"
	t.Toolkit.Description = "Generate videos with the MiniMax API."
	t.Toolkit.Timeout = 30 * time.Second
	for _, o := range opts {
		o(t)
	}
	if t.baseURL == "" {
		u, ok := VideoURLs[t.region]
		if !ok {
			u = VideoURLs["global_en"]
		}
		t.baseURL = u
	}
	t.baseURL = strings.TrimRight(t.baseURL, "/")
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableVideo {
		t.Toolkit.Register("GenerateVideo", "Generate a video from a text prompt.", t, t.GenerateVideo, GenerateVideoParams{})
	}
	return t
}

func (t *MiniMaxTools) queryURL() string {
	return strings.Replace(t.baseURL, "/v2/video_generation", "/v2/query/video_generation", 1)
}

// GenerateVideo generates a video and waits for its URL.
func (t *MiniMaxTools) GenerateVideo(p GenerateVideoParams) (interface{}, error) {
	if t.apiKey == "" {
		return "Please set the MINIMAX_API_KEY", nil
	}
	if p.Prompt == "" {
		return "Please provide a prompt", nil
	}
	if p.Resolution == "" {
		p.Resolution = "2K"
	}
	if p.Duration == 0 {
		p.Duration = 5
	}
	if p.Ratio == "" {
		p.Ratio = "16:9"
	}
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Bearer " + t.apiKey})
	ctx := context.Background()
	var created struct {
		TaskID string `json:"task_id"`
	}
	err := c.JSON(ctx, http.MethodPost, t.baseURL, nil, map[string]interface{}{
		"model":      t.model,
		"content":    []map[string]string{{"type": "text", "text": p.Prompt}},
		"resolution": p.Resolution, "duration": p.Duration, "ratio": p.Ratio,
	}, &created)
	if err != nil {
		return fmt.Sprintf("Error generating video: %v", err), nil
	}
	if created.TaskID == "" {
		return "Failed to generate video: No task ID returned", nil
	}
	deadline := time.Now().Add(t.maxWaitTime)
	for time.Until(deadline) >= minPollRequest {
		var resp struct {
			Task struct {
				Status  string `json:"status"`
				Content struct {
					URL string `json:"url"`
				} `json:"content"`
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"task"`
		}
		if err := c.JSON(ctx, http.MethodGet, t.queryURL()+"/"+created.TaskID, nil, nil, &resp); err != nil {
			return fmt.Sprintf("Error generating video: %v", err), nil
		}
		switch resp.Task.Status {
		case "succeeded":
			if resp.Task.Content.URL == "" {
				return "Failed to generate video: No video URL returned", nil
			}
			return mediaout.Result{
				Content: "Video generated successfully",
				Videos: []mediaout.Artifact{{ID: mediaout.NewID(), URL: resp.Task.Content.URL,
					OriginalPrompt: p.Prompt, MimeType: "video/mp4"}},
			}.String(), nil
		case "failed", "cancelled":
			msg := resp.Task.Error.Message
			if msg == "" {
				msg = resp.Task.Status
			}
			return fmt.Sprintf("Failed to generate video: %s", msg), nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := t.pollInterval
		if wait > remaining {
			wait = remaining
		}
		time.Sleep(wait)
	}
	return fmt.Sprintf("Video generation timed out after %v seconds", t.maxWaitTime.Seconds()), nil
}

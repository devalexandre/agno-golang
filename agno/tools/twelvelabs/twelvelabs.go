// Package twelvelabs ports Agno's TwelveLabsTools: video understanding with
// Pegasus (analyze) and multimodal embeddings with Marengo, via the TwelveLabs
// REST API (v1.3).
package twelvelabs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the TwelveLabs API root.
const DefaultBaseURL = "https://api.twelvelabs.io/v1.3"

// TwelveLabsTools exposes TwelveLabs video understanding to agents.
type TwelveLabsTools struct {
	toolkit.Toolkit
	apiKey            string
	baseURL           string
	httpClient        *http.Client
	analyzeModel      string
	embedModel        string
	maxTokens         int
	embedPollInterval time.Duration
	embedTimeout      time.Duration

	enableAnalyzeVideo bool
	enableEmbedText    bool
	enableEmbedVideo   bool
}

// Option configures TwelveLabsTools.
type Option func(*TwelveLabsTools)

// WithAPIKey sets the API key (default TWELVELABS_API_KEY).
func WithAPIKey(k string) Option { return func(t *TwelveLabsTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *TwelveLabsTools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *TwelveLabsTools) { t.httpClient = c } }

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option { return func(t *TwelveLabsTools) { t.Toolkit.Timeout = d } }

// WithAnalyzeModel sets the Pegasus model (default pegasus1.5).
func WithAnalyzeModel(m string) Option { return func(t *TwelveLabsTools) { t.analyzeModel = m } }

// WithEmbedModel sets the Marengo model (default marengo3.0).
func WithEmbedModel(m string) Option { return func(t *TwelveLabsTools) { t.embedModel = m } }

// WithMaxTokens sets the AnalyzeVideo response limit (default 2048).
func WithMaxTokens(n int) Option { return func(t *TwelveLabsTools) { t.maxTokens = n } }

// WithEmbedPollInterval sets the EmbedVideo polling interval (default 5s).
func WithEmbedPollInterval(d time.Duration) Option {
	return func(t *TwelveLabsTools) { t.embedPollInterval = d }
}

// WithEmbedTimeout caps EmbedVideo waiting (default 300s).
func WithEmbedTimeout(d time.Duration) Option { return func(t *TwelveLabsTools) { t.embedTimeout = d } }

// WithAnalyzeVideo enables/disables AnalyzeVideo (default true).
func WithAnalyzeVideo(on bool) Option { return func(t *TwelveLabsTools) { t.enableAnalyzeVideo = on } }

// WithEmbedText enables/disables EmbedText (default true).
func WithEmbedText(on bool) Option { return func(t *TwelveLabsTools) { t.enableEmbedText = on } }

// WithEmbedVideo enables/disables EmbedVideo (default false: long-running).
func WithEmbedVideo(on bool) Option { return func(t *TwelveLabsTools) { t.enableEmbedVideo = on } }

// WithAll enables every tool.
func WithAll() Option {
	return func(t *TwelveLabsTools) {
		t.enableAnalyzeVideo, t.enableEmbedText, t.enableEmbedVideo = true, true, true
	}
}

// AnalyzeVideoParams are the parameters of AnalyzeVideo.
type AnalyzeVideoParams struct {
	VideoURL string `json:"video_url" description:"A publicly accessible URL of the video to analyze." required:"true"`
	Prompt   string `json:"prompt" description:"The natural-language question or instruction about the video." required:"true"`
}

// EmbedTextParams are the parameters of EmbedText.
type EmbedTextParams struct {
	Text string `json:"text" description:"The text to embed." required:"true"`
}

// EmbedVideoParams are the parameters of EmbedVideo.
type EmbedVideoParams struct {
	VideoURL string `json:"video_url" description:"A publicly accessible URL of the video to embed." required:"true"`
}

// NewTwelveLabsTools creates the toolkit.
func NewTwelveLabsTools(opts ...Option) *TwelveLabsTools {
	t := &TwelveLabsTools{
		apiKey: os.Getenv("TWELVELABS_API_KEY"), baseURL: DefaultBaseURL,
		analyzeModel: "pegasus1.5", embedModel: "marengo3.0", maxTokens: 2048,
		embedPollInterval: 5 * time.Second, embedTimeout: 300 * time.Second,
		enableAnalyzeVideo: true, enableEmbedText: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "twelvelabs_tools"
	t.Toolkit.Description = "Video understanding (Pegasus) and multimodal embeddings (Marengo) with TwelveLabs."
	t.Toolkit.Timeout = 2 * time.Minute
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableAnalyzeVideo {
		t.Toolkit.Register("AnalyzeVideo", "Analyze a video and answer a question about it using the Pegasus model.", t, t.AnalyzeVideo, AnalyzeVideoParams{})
	}
	if t.enableEmbedText {
		t.Toolkit.Register("EmbedText", "Generate a multimodal embedding for the given text using the Marengo model. The vector lives in the same latent space as TwelveLabs video, audio and image embeddings, so it can be used to search a video corpus by text.", t, t.EmbedText, EmbedTextParams{})
	}
	if t.enableEmbedVideo {
		t.Toolkit.Register("EmbedVideo", "Generate Marengo multimodal embeddings for a video (one vector per 2-10s segment). Returns a compact JSON summary: model, dimensions, segment count and per-segment offsets/scope (raw vectors are not returned).", t, t.EmbedVideo, EmbedVideoParams{})
	}
	return t
}

func (t *TwelveLabsTools) client() (*restapi.Client, error) {
	if t.apiKey == "" {
		return nil, fmt.Errorf("No TwelveLabs API key provided. Set TWELVELABS_API_KEY or pass api_key.")
	}
	return restapi.New(t.baseURL, t.httpClient, map[string]string{"x-api-key": t.apiKey}), nil
}

// AnalyzeVideo answers a question about a video.
func (t *TwelveLabsTools) AnalyzeVideo(p AnalyzeVideoParams) (interface{}, error) {
	if p.VideoURL == "" {
		return "No video_url provided", nil
	}
	if p.Prompt == "" {
		return "No prompt provided", nil
	}
	c, err := t.client()
	if err != nil {
		return fmt.Sprintf("Error analyzing video: %v", err), nil
	}
	var resp struct {
		Data string `json:"data"`
	}
	err = c.JSON(context.Background(), http.MethodPost, "/analyze", nil, map[string]interface{}{
		"model_name": t.analyzeModel,
		"video":      map[string]string{"type": "url", "url": p.VideoURL},
		"prompt":     p.Prompt, "max_tokens": t.maxTokens, "stream": false,
	}, &resp)
	if err != nil {
		return fmt.Sprintf("Error analyzing video: %v", err), nil
	}
	if resp.Data == "" {
		return "No analysis returned", nil
	}
	return resp.Data, nil
}

type segment struct {
	Float          []float64 `json:"float"`
	StartOffsetSec float64   `json:"start_offset_sec"`
	EndOffsetSec   float64   `json:"end_offset_sec"`
	EmbeddingScope string    `json:"embedding_scope"`
}

// EmbedText embeds text with Marengo.
func (t *TwelveLabsTools) EmbedText(p EmbedTextParams) (interface{}, error) {
	if p.Text == "" {
		return "No text provided", nil
	}
	c, err := t.client()
	if err != nil {
		return fmt.Sprintf("Error embedding text: %v", err), nil
	}
	data, err := c.Multipart(context.Background(), http.MethodPost, "/embed",
		map[string]string{"model_name": t.embedModel, "text": p.Text}, nil)
	if err != nil {
		return fmt.Sprintf("Error embedding text: %v", err), nil
	}
	var resp struct {
		TextEmbedding *struct {
			Segments []segment `json:"segments"`
		} `json:"text_embedding"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Sprintf("Error embedding text: %v", err), nil
	}
	if resp.TextEmbedding == nil || len(resp.TextEmbedding.Segments) == 0 || len(resp.TextEmbedding.Segments[0].Float) == 0 {
		return "No embedding returned", nil
	}
	vec := resp.TextEmbedding.Segments[0].Float
	b, _ := json.Marshal(map[string]interface{}{"model": t.embedModel, "dimensions": len(vec), "embedding": vec})
	return string(b), nil
}

// EmbedVideo embeds a video and returns a segmentation summary.
func (t *TwelveLabsTools) EmbedVideo(p EmbedVideoParams) (interface{}, error) {
	if p.VideoURL == "" {
		return "No video_url provided", nil
	}
	c, err := t.client()
	if err != nil {
		return fmt.Sprintf("Error embedding video: %v", err), nil
	}
	ctx := context.Background()
	data, err := c.Multipart(ctx, http.MethodPost, "/embed/tasks",
		map[string]string{"model_name": t.embedModel, "video_url": p.VideoURL}, nil)
	if err != nil {
		return fmt.Sprintf("Error embedding video: %v", err), nil
	}
	var task struct {
		ID string `json:"_id"`
	}
	_ = json.Unmarshal(data, &task)
	if task.ID == "" {
		return "No embedding task id returned", nil
	}
	status := func() (string, error) {
		var s struct {
			Status string `json:"status"`
		}
		err := c.JSON(ctx, http.MethodGet, "/embed/tasks/"+task.ID+"/status", nil, nil, &s)
		return s.Status, err
	}
	deadline := time.Now().Add(t.embedTimeout)
	st, err := status()
	for err == nil && st != "ready" && st != "failed" {
		if !time.Now().Before(deadline) {
			return fmt.Sprintf("Video embedding task timed out after %.0fs (status: %s)", t.embedTimeout.Seconds(), st), nil
		}
		time.Sleep(t.embedPollInterval)
		st, err = status()
	}
	if err != nil {
		return fmt.Sprintf("Error embedding video: %v", err), nil
	}
	if st != "ready" {
		return fmt.Sprintf("Video embedding task did not complete (status: %s)", st), nil
	}
	var result struct {
		VideoEmbedding *struct {
			Segments []segment `json:"segments"`
		} `json:"video_embedding"`
	}
	if err := c.JSON(ctx, http.MethodGet, "/embed/tasks/"+task.ID, nil, nil, &result); err != nil {
		return fmt.Sprintf("Error embedding video: %v", err), nil
	}
	if result.VideoEmbedding == nil {
		return "No embedding returned", nil
	}
	dims := 0
	segs := []map[string]interface{}{}
	for _, s := range result.VideoEmbedding.Segments {
		if len(s.Float) == 0 {
			continue
		}
		if dims == 0 {
			dims = len(s.Float)
		}
		segs = append(segs, map[string]interface{}{
			"start_offset_sec": s.StartOffsetSec, "end_offset_sec": s.EndOffsetSec, "embedding_scope": s.EmbeddingScope,
		})
	}
	if len(segs) == 0 {
		return "No embedding returned", nil
	}
	b, _ := json.Marshal(map[string]interface{}{
		"model": t.embedModel, "dimensions": dims, "num_segments": len(segs), "segments": segs,
	})
	return string(b), nil
}

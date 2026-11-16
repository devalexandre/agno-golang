// Package nanobanana ports Agno's NanoBananaTools: image generation with
// Gemini image models ("Nano Banana") through the Gemini generateContent REST API.
package nanobanana

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultBaseURL is the Gemini API base URL.
const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// AllowedModels lists the supported image models.
var AllowedModels = []string{"gemini-2.5-flash-image"}

// AllowedRatios lists the supported aspect ratios.
var AllowedRatios = []string{"1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"}

// Image is a generated image artifact.
type Image struct {
	ID             string `json:"id"`
	MimeType       string `json:"mime_type"`
	OriginalPrompt string `json:"original_prompt"`
	Content        []byte `json:"-"`
}

// NanoBananaTools generates images with Gemini.
type NanoBananaTools struct {
	toolkit.Toolkit
	apiKey      string
	baseURL     string
	model       string
	aspectRatio string
	httpClient  *http.Client

	enableCreateImage bool

	mu     sync.Mutex
	images []Image
}

// Option configures NanoBananaTools.
type Option func(*NanoBananaTools)

// WithAPIKey sets the API key (default GOOGLE_API_KEY).
func WithAPIKey(k string) Option { return func(t *NanoBananaTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *NanoBananaTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *NanoBananaTools) { t.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(t *NanoBananaTools) { t.Toolkit.Timeout = d } }

// WithModel sets the model (default gemini-2.5-flash-image).
func WithModel(m string) Option { return func(t *NanoBananaTools) { t.model = m } }

// WithAspectRatio sets the aspect ratio (default 1:1).
func WithAspectRatio(r string) Option { return func(t *NanoBananaTools) { t.aspectRatio = r } }

// WithCreateImage toggles CreateImage (default true).
func WithCreateImage(b bool) Option { return func(t *NanoBananaTools) { t.enableCreateImage = b } }

// CreateImageParams are the CreateImage parameters.
type CreateImageParams struct {
	Prompt string `json:"prompt" description:"Text prompt describing the image to generate." required:"true"`
}

// NewNanoBananaTools creates the toolkit. Like Python, it fails on an
// unsupported model or aspect ratio, or when no API key is available.
func NewNanoBananaTools(opts ...Option) (*NanoBananaTools, error) {
	t := &NanoBananaTools{
		apiKey:            os.Getenv("GOOGLE_API_KEY"),
		baseURL:           DefaultBaseURL,
		model:             "gemini-2.5-flash-image",
		aspectRatio:       "1:1",
		enableCreateImage: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "nano_banana"
	t.Toolkit.Description = "Generate images from text prompts with Gemini (Nano Banana)."
	t.Toolkit.Timeout = 2 * time.Minute
	for _, o := range opts {
		o(t)
	}
	if !contains(AllowedModels, t.model) {
		return nil, fmt.Errorf("invalid model '%s'. Supported: %s", t.model, strings.Join(AllowedModels, ", "))
	}
	if !contains(AllowedRatios, t.aspectRatio) {
		return nil, fmt.Errorf("invalid aspect_ratio '%s'. Supported: %s", t.aspectRatio, strings.Join(AllowedRatios, ", "))
	}
	if t.apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY not set. Export it: `export GOOGLE_API_KEY=<your-key>`")
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableCreateImage {
		t.Toolkit.Register("CreateImage", "Generate an image from a text prompt.", t, t.CreateImage, CreateImageParams{})
	}
	return t, nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// Images returns the images generated so far.
func (t *NanoBananaTools) Images() []Image {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Image(nil), t.images...)
}

// CreateImage generates an image from a prompt.
func (t *NanoBananaTools) CreateImage(p CreateImageParams) (interface{}, error) {
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"x-goog-api-key": t.apiKey})
	body := map[string]interface{}{
		"contents": []map[string]interface{}{{"parts": []map[string]string{{"text": p.Prompt}}}},
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"IMAGE"},
			"imageConfig":        map[string]string{"aspectRatio": t.aspectRatio},
		},
	}
	var resp struct {
		Candidates []struct {
			Content *struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	path := "/models/" + url.PathEscape(t.model) + ":generateContent"
	if err := c.JSON(context.Background(), http.MethodPost, path, nil, body, &resp); err != nil {
		return fmt.Sprintf("Error generating image: %v", err), nil
	}
	if len(resp.Candidates) == 0 {
		return "No images were generated in the response", nil
	}
	var sb strings.Builder
	var generated []Image
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				sb.WriteString(part.Text + "\n")
			}
			if part.InlineData != nil && part.InlineData.Data != "" {
				data, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
				if err != nil {
					fmt.Fprintf(&sb, "Failed to process image: %v\n", err)
					continue
				}
				mime := part.InlineData.MimeType
				if mime == "" {
					mime = "image/png"
				}
				img := Image{ID: uuid.NewString(), MimeType: mime, OriginalPrompt: p.Prompt, Content: data}
				generated = append(generated, img)
				fmt.Fprintf(&sb, "Image generated successfully (ID: %s).\n", img.ID)
			}
		}
	}
	msg := strings.TrimSpace(sb.String())
	if len(generated) == 0 {
		if msg == "" {
			msg = "No images were generated"
		}
		return msg, nil
	}
	t.mu.Lock()
	t.images = append(t.images, generated...)
	t.mu.Unlock()
	if msg == "" {
		msg = "Image(s) generated successfully"
	}
	return msg, nil
}

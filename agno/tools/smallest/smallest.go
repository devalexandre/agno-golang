// Package smallest ports Agno's SmallestTools: voice listing and text-to-speech
// with Smallest AI Lightning models.
package smallest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/mediaout"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultTTSURL is the Smallest AI TTS endpoint.
const DefaultTTSURL = "https://api.smallest.ai/waves/v1/tts"

// VoicesURLs maps each model to its voice catalog route. The standard catalog
// route retires on 2026-11-30, so both models use the Pro catalog.
var VoicesURLs = map[string]string{
	"lightning_v3.1":     "https://api.smallest.ai/waves/v1/lightning-v3.1-pro/get_voices",
	"lightning_v3.1_pro": "https://api.smallest.ai/waves/v1/lightning-v3.1-pro/get_voices",
}

var mimeTypes = map[string]string{
	"wav": "audio/wav", "mp3": "audio/mpeg", "pcm": "audio/wav", "ulaw": "audio/basic", "alaw": "audio/x-alaw-basic",
}

// SmallestTools converts text to speech with Smallest AI.
type SmallestTools struct {
	toolkit.Toolkit
	apiKey          string
	voiceID         string
	model           string
	language        string
	sampleRate      int
	speed           float64
	outputFormat    string
	targetDirectory string
	baseURL         string
	voicesURL       string
	httpClient      *http.Client

	enableGetVoices    bool
	enableTextToSpeech bool
}

// Option configures SmallestTools.
type Option func(*SmallestTools)

// WithAPIKey sets the API key (default SMALLEST_API_KEY).
func WithAPIKey(k string) Option { return func(t *SmallestTools) { t.apiKey = k } }

// WithVoiceID sets the default voice (default kaitlyn).
func WithVoiceID(v string) Option { return func(t *SmallestTools) { t.voiceID = v } }

// WithModel sets the model: lightning_v3.1_pro (default) or lightning_v3.1.
func WithModel(m string) Option { return func(t *SmallestTools) { t.model = m } }

// WithLanguage sets the language (default en).
func WithLanguage(l string) Option { return func(t *SmallestTools) { t.language = l } }

// WithSampleRate sets the sample rate (default 24000).
func WithSampleRate(r int) Option { return func(t *SmallestTools) { t.sampleRate = r } }

// WithSpeed sets the speaking speed (default 1.0).
func WithSpeed(s float64) Option { return func(t *SmallestTools) { t.speed = s } }

// WithOutputFormat sets wav (default), mp3, pcm, ulaw or alaw.
func WithOutputFormat(f string) Option { return func(t *SmallestTools) { t.outputFormat = f } }

// WithTargetDirectory saves generated audio there (default: OS temp dir).
func WithTargetDirectory(d string) Option { return func(t *SmallestTools) { t.targetDirectory = d } }

// WithBaseURL overrides the TTS endpoint.
func WithBaseURL(u string) Option { return func(t *SmallestTools) { t.baseURL = u } }

// WithVoicesURL overrides the voice catalog endpoint.
func WithVoicesURL(u string) Option { return func(t *SmallestTools) { t.voicesURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *SmallestTools) { t.httpClient = c } }

// WithTimeout sets the request timeout (default 30s).
func WithTimeout(d time.Duration) Option { return func(t *SmallestTools) { t.Toolkit.Timeout = d } }

// WithGetVoices enables/disables GetVoices (default true).
func WithGetVoices(on bool) Option { return func(t *SmallestTools) { t.enableGetVoices = on } }

// WithTextToSpeech enables/disables TextToSpeech (default true).
func WithTextToSpeech(on bool) Option { return func(t *SmallestTools) { t.enableTextToSpeech = on } }

// WithAll enables every tool.
func WithAll() Option {
	return func(t *SmallestTools) { t.enableGetVoices, t.enableTextToSpeech = true, true }
}

// GetVoicesParams has no parameters.
type GetVoicesParams struct{}

// TextToSpeechParams are the parameters of TextToSpeech.
type TextToSpeechParams struct {
	Prompt  string `json:"prompt" description:"Text to generate audio from." required:"true"`
	VoiceID string `json:"voice_id,omitempty" description:"The ID of the voice to use. If empty, uses the default voice configured in the tool. The voice must belong to the configured model's pool."`
}

// NewSmallestTools creates the toolkit. An unknown model falls back to lightning_v3.1_pro.
func NewSmallestTools(opts ...Option) *SmallestTools {
	t := &SmallestTools{
		apiKey: os.Getenv("SMALLEST_API_KEY"), voiceID: "kaitlyn", model: "lightning_v3.1_pro",
		language: "en", sampleRate: 24000, speed: 1.0, outputFormat: "wav", baseURL: DefaultTTSURL,
		enableGetVoices: true, enableTextToSpeech: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "smallest_tools"
	t.Toolkit.Description = "Text-to-speech with Smallest AI Lightning models."
	t.Toolkit.Timeout = 30 * time.Second
	for _, o := range opts {
		o(t)
	}
	if _, ok := VoicesURLs[t.model]; !ok {
		t.model = "lightning_v3.1_pro"
	}
	if t.voicesURL == "" {
		t.voicesURL = VoicesURLs[t.model]
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableGetVoices {
		t.Toolkit.Register("GetVoices", "Get voices from the Lightning v3.1 Pro catalog. Returns a JSON list of voices with their metadata (id, name, gender, accent, languages).", t, t.GetVoices, GetVoicesParams{})
	}
	if t.enableTextToSpeech {
		t.Toolkit.Register("TextToSpeech", "Convert text to natural speech using Smallest AI Lightning models.", t, t.TextToSpeech, TextToSpeechParams{})
	}
	return t
}

func (t *SmallestTools) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + t.apiKey, "X-Source": "agno"}
}

func firstNonEmpty(vals ...interface{}) interface{} {
	for _, v := range vals {
		if v != nil && v != "" {
			return v
		}
	}
	return nil
}

// GetVoices lists the available voices.
func (t *SmallestTools) GetVoices(GetVoicesParams) (interface{}, error) {
	c := restapi.New("", t.httpClient, t.headers())
	data, err := c.Do(context.Background(), http.MethodGet, t.voicesURL, nil, nil)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	voices := parsed
	if m, ok := parsed.(map[string]interface{}); ok {
		if v, ok := m["voices"]; ok {
			voices = v
		} else if v, ok := m["data"]; ok {
			voices = v
		}
	}
	list, ok := voices.([]interface{})
	if !ok {
		b, _ := json.Marshal(voices)
		return string(b), nil
	}
	result := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		v, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		tags, _ := v["tags"].(map[string]interface{})
		if tags == nil {
			tags = map[string]interface{}{}
		}
		result = append(result, map[string]interface{}{
			"id":        firstNonEmpty(v["voiceId"], v["voice_id"], v["id"]),
			"name":      firstNonEmpty(v["displayName"], v["name"]),
			"gender":    firstNonEmpty(tags["gender"], v["gender"]),
			"accent":    firstNonEmpty(tags["accent"], v["accent"]),
			"languages": firstNonEmpty(tags["language"], v["languages"]),
		})
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

// TextToSpeech converts text to speech and saves the audio.
func (t *SmallestTools) TextToSpeech(p TextToSpeechParams) (interface{}, error) {
	mime, ok := mimeTypes[t.outputFormat]
	if !ok {
		mime = "audio/wav"
	}
	voice := p.VoiceID
	if voice == "" {
		voice = t.voiceID
	}
	body, _ := json.Marshal(map[string]interface{}{
		"text": p.Prompt, "voice_id": voice, "model": t.model, "language": t.language,
		"sample_rate": t.sampleRate, "speed": t.speed, "output_format": t.outputFormat,
	})
	req, err := http.NewRequest(http.MethodPost, t.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	for k, v := range t.headers() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", mime)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	defer resp.Body.Close()
	audio, err := io.ReadAll(io.LimitReader(resp.Body, restapi.MaxResponseBytes))
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("Error: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(audio))), nil
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "audio") && !strings.Contains(ct, "octet-stream") {
		return fmt.Sprintf("Error: Expected audio response but got content-type '%s': %s", ct, string(audio)), nil
	}
	id := mediaout.NewID()
	path, err := mediaout.Save(t.targetDirectory, id, t.outputFormat, audio)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return mediaout.Result{
		Content: "Audio generated successfully",
		Audios: []mediaout.Artifact{{ID: id, Path: path, MimeType: mime, Format: t.outputFormat,
			SampleRate: t.sampleRate}},
	}.String(), nil
}

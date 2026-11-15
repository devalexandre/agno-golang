// Package desivocal ports Agno's DesiVocalTools: Indian-language voices and
// text-to-speech through the DesiVocal REST API.
package desivocal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/mediaout"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the DesiVocal TTS API root.
const DefaultBaseURL = "https://prod-api2.desivocal.com/dv/api/v0/tts_api"

// DesiVocalTools generates speech with DesiVocal.
type DesiVocalTools struct {
	toolkit.Toolkit
	apiKey     string
	voiceID    string
	baseURL    string
	httpClient *http.Client

	enableGetVoices    bool
	enableTextToSpeech bool
}

// Option configures DesiVocalTools.
type Option func(*DesiVocalTools)

// WithAPIKey sets the API key (default DESI_VOCAL_API_KEY).
func WithAPIKey(k string) Option { return func(t *DesiVocalTools) { t.apiKey = k } }

// WithVoiceID sets the default voice.
func WithVoiceID(v string) Option { return func(t *DesiVocalTools) { t.voiceID = v } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *DesiVocalTools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *DesiVocalTools) { t.httpClient = c } }

// WithTimeout sets the request timeout (default 30s).
func WithTimeout(d time.Duration) Option { return func(t *DesiVocalTools) { t.Toolkit.Timeout = d } }

// WithGetVoices enables/disables GetVoices (default true).
func WithGetVoices(on bool) Option { return func(t *DesiVocalTools) { t.enableGetVoices = on } }

// WithTextToSpeech enables/disables TextToSpeech (default true).
func WithTextToSpeech(on bool) Option { return func(t *DesiVocalTools) { t.enableTextToSpeech = on } }

// WithAll enables every tool.
func WithAll() Option {
	return func(t *DesiVocalTools) { t.enableGetVoices, t.enableTextToSpeech = true, true }
}

// GetVoicesParams has no parameters.
type GetVoicesParams struct{}

// TextToSpeechParams are the parameters of TextToSpeech.
type TextToSpeechParams struct {
	Prompt  string `json:"prompt" description:"The text to generate audio from." required:"true"`
	VoiceID string `json:"voice_id,omitempty" description:"Optional voice ID; defaults to the toolkit voice."`
}

// NewDesiVocalTools creates the toolkit.
func NewDesiVocalTools(opts ...Option) *DesiVocalTools {
	t := &DesiVocalTools{
		apiKey: os.Getenv("DESI_VOCAL_API_KEY"), voiceID: "f27d74e5-ea71-4697-be3e-f04bbd80c1a8",
		baseURL: DefaultBaseURL, enableGetVoices: true, enableTextToSpeech: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "desi_vocal_tools"
	t.Toolkit.Description = "Text-to-speech with DesiVocal voices."
	t.Toolkit.Timeout = 30 * time.Second
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableGetVoices {
		t.Toolkit.Register("GetVoices", "Use this function to get all the voices available. Returns a list of voices that have an ID, name and description.", t, t.GetVoices, GetVoicesParams{})
	}
	if t.enableTextToSpeech {
		t.Toolkit.Register("TextToSpeech", "Use this function to generate audio from text.", t, t.TextToSpeech, TextToSpeechParams{})
	}
	return t
}

type voiceInfo struct {
	Name        string            `json:"name"`
	AudioGender string            `json:"audio_gender"`
	VoiceType   string            `json:"voice_type"`
	Languages   []string          `json:"languages"`
	PreviewPath map[string]string `json:"preview_path"`
}

// GetVoices lists the available voices.
func (t *DesiVocalTools) GetVoices(GetVoicesParams) (interface{}, error) {
	var voices map[string]voiceInfo
	c := restapi.New(t.baseURL, t.httpClient, nil)
	if err := c.JSON(context.Background(), http.MethodGet, "/voices", nil, nil, &voices); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	ids := make([]string, 0, len(voices))
	for id := range voices {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		v := voices[id]
		var preview interface{}
		keys := make([]string, 0, len(v.PreviewPath))
		for k := range v.PreviewPath {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			preview = v.PreviewPath[keys[0]]
		}
		out = append(out, map[string]interface{}{
			"id": id, "name": v.Name, "gender": v.AudioGender, "type": v.VoiceType,
			"language": strings.Join(v.Languages, ", "), "preview_url": preview,
		})
	}
	return restapi.ToJSON(out), nil
}

// TextToSpeech generates audio from text and returns its URL.
func (t *DesiVocalTools) TextToSpeech(p TextToSpeechParams) (interface{}, error) {
	voice := p.VoiceID
	if voice == "" {
		voice = t.voiceID
	}
	c := restapi.New(t.baseURL, t.httpClient, map[string]string{"X_API_KEY": t.apiKey})
	var resp struct {
		S3Path string `json:"s3_path"`
	}
	if err := c.JSON(context.Background(), http.MethodPost, "/generate", nil,
		map[string]string{"text": p.Prompt, "voice_id": voice}, &resp); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	if resp.S3Path == "" {
		return "Error: 's3_path' missing from response", nil
	}
	return mediaout.Result{
		Content: fmt.Sprintf("Audio generated successfully: %s", resp.S3Path),
		Audios:  []mediaout.Artifact{{ID: mediaout.NewID(), URL: resp.S3Path}},
	}.String(), nil
}

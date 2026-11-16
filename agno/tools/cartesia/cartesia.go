// Package cartesia ports Agno's CartesiaTools: text-to-speech, voice listing
// and voice localization through the Cartesia REST API.
package cartesia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultBaseURL is the Cartesia API base URL.
const DefaultBaseURL = "https://api.cartesia.ai"

// APIVersion is sent in the Cartesia-Version header.
const APIVersion = "2025-04-16"

// Audio is a generated audio artifact.
type Audio struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Content  []byte `json:"-"`
}

// CartesiaTools wraps the Cartesia API.
type CartesiaTools struct {
	toolkit.Toolkit
	apiKey         string
	baseURL        string
	modelID        string
	defaultVoiceID string
	httpClient     *http.Client

	enableTextToSpeech  bool
	enableListVoices    bool
	enableLocalizeVoice bool

	mu     sync.Mutex
	audios []Audio
}

// Option configures CartesiaTools.
type Option func(*CartesiaTools)

// WithAPIKey sets the API key (default CARTESIA_API_KEY).
func WithAPIKey(k string) Option { return func(t *CartesiaTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *CartesiaTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *CartesiaTools) { t.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(t *CartesiaTools) { t.Toolkit.Timeout = d } }

// WithModelID sets the TTS model (default sonic-3.6).
func WithModelID(id string) Option { return func(t *CartesiaTools) { t.modelID = id } }

// WithDefaultVoiceID sets the default voice.
func WithDefaultVoiceID(id string) Option { return func(t *CartesiaTools) { t.defaultVoiceID = id } }

// WithTextToSpeech toggles TextToSpeech (default true).
func WithTextToSpeech(b bool) Option { return func(t *CartesiaTools) { t.enableTextToSpeech = b } }

// WithListVoices toggles ListVoices (default true).
func WithListVoices(b bool) Option { return func(t *CartesiaTools) { t.enableListVoices = b } }

// WithLocalizeVoice toggles LocalizeVoice (default false).
func WithLocalizeVoice(b bool) Option { return func(t *CartesiaTools) { t.enableLocalizeVoice = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(t *CartesiaTools) {
		t.enableTextToSpeech, t.enableListVoices, t.enableLocalizeVoice = true, true, true
	}
}

// ListVoicesParams has no parameters.
type ListVoicesParams struct{}

// LocalizeVoiceParams are the LocalizeVoice parameters.
type LocalizeVoiceParams struct {
	Name                  string `json:"name" description:"The desired name for the new localized voice." required:"true"`
	Description           string `json:"description" description:"The description for the new localized voice." required:"true"`
	Language              string `json:"language" description:"The target language code (e.g., 'fr', 'es')." required:"true"`
	OriginalSpeakerGender string `json:"original_speaker_gender" description:"The gender of the original speaker (\"male\" or \"female\")." required:"true"`
	VoiceID               string `json:"voice_id,omitempty" description:"The ID of an existing voice to use as the base. Defaults to the configured default voice."`
}

// TextToSpeechParams are the TextToSpeech parameters.
type TextToSpeechParams struct {
	Transcript string `json:"transcript" description:"The text to convert to speech" required:"true"`
	VoiceID    string `json:"voice_id,omitempty" description:"The ID of the voice to use. Defaults to the configured default voice."`
}

// NewCartesiaTools creates the toolkit.
func NewCartesiaTools(opts ...Option) *CartesiaTools {
	t := &CartesiaTools{
		apiKey:             os.Getenv("CARTESIA_API_KEY"),
		baseURL:            DefaultBaseURL,
		modelID:            "sonic-3.6",
		defaultVoiceID:     "78ab82d5-25be-4f7d-82b3-7ad64e5b85b2",
		enableTextToSpeech: true,
		enableListVoices:   true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "cartesia_tools"
	t.Toolkit.Description = "Text-to-speech and voice management with Cartesia."
	t.Toolkit.Timeout = 2 * time.Minute
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableLocalizeVoice {
		t.Toolkit.Register("LocalizeVoice", "Create a new voice localized to a different language. Returns JSON with the new voice information, including its 'id'.", t, t.LocalizeVoice, LocalizeVoiceParams{})
	}
	if t.enableTextToSpeech {
		t.Toolkit.Register("TextToSpeech", "Convert text to speech.", t, t.TextToSpeech, TextToSpeechParams{})
	}
	if t.enableListVoices {
		t.Toolkit.Register("ListVoices", "List available voices from Cartesia (first page). Returns JSON with id, name, description and language of each voice.", t, t.ListVoices, ListVoicesParams{})
	}
	return t
}

func (t *CartesiaTools) client() *restapi.Client {
	return restapi.New(t.baseURL, t.httpClient, map[string]string{
		"X-API-Key":        t.apiKey,
		"Authorization":    "Bearer " + t.apiKey,
		"Cartesia-Version": APIVersion,
	})
}

// Audios returns the audio artifacts generated so far.
func (t *CartesiaTools) Audios() []Audio {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Audio(nil), t.audios...)
}

// ListVoices lists voices (first page).
func (t *CartesiaTools) ListVoices(ListVoicesParams) (interface{}, error) {
	data, err := t.client().Do(context.Background(), http.MethodGet, "/voices", nil, nil)
	if err != nil {
		return restapi.ToJSON(map[string]string{"error": err.Error(), "detail": "Error occurred in list_voices function."}), nil
	}
	var items []map[string]interface{}
	var page struct {
		Data  []map[string]interface{} `json:"data"`
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		if err := json.Unmarshal(data, &page); err != nil {
			return restapi.ToJSON(map[string]string{"error": err.Error(), "detail": "Error occurred in list_voices function."}), nil
		}
		items = page.Data
		if items == nil {
			items = page.Items
		}
	}
	out := []map[string]interface{}{}
	for _, v := range items {
		if v["id"] == nil || v["id"] == "" {
			continue
		}
		out = append(out, map[string]interface{}{
			"id": v["id"], "name": v["name"], "description": v["description"], "language": v["language"],
		})
	}
	return restapi.ToJSON(out), nil
}

// LocalizeVoice creates a voice localized to another language.
func (t *CartesiaTools) LocalizeVoice(p LocalizeVoiceParams) (interface{}, error) {
	voiceID := p.VoiceID
	if voiceID == "" {
		voiceID = t.defaultVoiceID
	}
	data, err := t.client().Do(context.Background(), http.MethodPost, "/voices/localize", nil, map[string]string{
		"voice_id":                voiceID,
		"name":                    p.Name,
		"description":             p.Description,
		"language":                p.Language,
		"original_speaker_gender": p.OriginalSpeakerGender,
	})
	if err != nil {
		return restapi.ToJSON(map[string]string{"error": err.Error(), "type": fmt.Sprintf("%T", err)}), nil
	}
	return restapi.Pretty(data), nil
}

// TextToSpeech converts a transcript to MP3 speech.
func (t *CartesiaTools) TextToSpeech(p TextToSpeechParams) (interface{}, error) {
	voiceID := p.VoiceID
	if voiceID == "" {
		voiceID = t.defaultVoiceID
	}
	c := t.client()
	c.Headers["Accept"] = "audio/mpeg"
	data, err := c.Do(context.Background(), http.MethodPost, "/tts/bytes", nil, map[string]interface{}{
		"model_id":   t.modelID,
		"transcript": p.Transcript,
		"voice":      map[string]string{"mode": "id", "id": voiceID},
		"output_format": map[string]interface{}{
			"container": "mp3", "sample_rate": 44100, "bit_rate": 128000, "encoding": "mp3",
		},
	})
	if err != nil {
		return fmt.Sprintf("Error generating speech: %v", err), nil
	}
	a := Audio{ID: uuid.NewString(), MimeType: "audio/mpeg", Content: data}
	t.mu.Lock()
	t.audios = append(t.audios, a)
	t.mu.Unlock()
	return restapi.ToJSON(map[string]interface{}{
		"content": "Audio generated and attached successfully.", "audio_id": a.ID, "mime_type": a.MimeType, "size_bytes": len(data),
	}), nil
}

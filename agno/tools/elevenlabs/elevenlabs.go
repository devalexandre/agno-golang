// Package elevenlabs ports Agno's ElevenLabsTools: text-to-speech, sound
// effects and voice listing through the ElevenLabs REST API.
package elevenlabs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/google/uuid"
)

// DefaultBaseURL is the ElevenLabs API base URL.
const DefaultBaseURL = "https://api.elevenlabs.io"

// Audio is a generated audio artifact.
type Audio struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	Path     string `json:"path,omitempty"`
	Content  []byte `json:"-"`
}

// ElevenLabsTools wraps the ElevenLabs API.
type ElevenLabsTools struct {
	toolkit.Toolkit
	apiKey          string
	baseURL         string
	voiceID         string
	modelID         string
	outputFormat    string
	targetDirectory string
	httpClient      *http.Client

	enableGetVoices           bool
	enableGenerateSoundEffect bool
	enableTextToSpeech        bool

	mu     sync.Mutex
	audios []Audio
}

// Option configures ElevenLabsTools.
type Option func(*ElevenLabsTools)

// WithAPIKey sets the API key (default ELEVEN_LABS_API_KEY).
func WithAPIKey(k string) Option { return func(t *ElevenLabsTools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *ElevenLabsTools) { t.baseURL = u } }

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *ElevenLabsTools) { t.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(t *ElevenLabsTools) { t.Toolkit.Timeout = d } }

// WithVoiceID sets the voice used for text-to-speech (default JBFqnCBsd6RMkjVDRZzb).
func WithVoiceID(id string) Option { return func(t *ElevenLabsTools) { t.voiceID = id } }

// WithModelID sets the TTS model (default eleven_multilingual_v2).
func WithModelID(id string) Option { return func(t *ElevenLabsTools) { t.modelID = id } }

// WithOutputFormat sets the audio output format (default mp3_44100_64).
func WithOutputFormat(f string) Option { return func(t *ElevenLabsTools) { t.outputFormat = f } }

// WithTargetDirectory saves generated audio files into dir.
func WithTargetDirectory(dir string) Option {
	return func(t *ElevenLabsTools) { t.targetDirectory = dir }
}

// WithGetVoices toggles GetVoices (default true).
func WithGetVoices(b bool) Option { return func(t *ElevenLabsTools) { t.enableGetVoices = b } }

// WithGenerateSoundEffect toggles GenerateSoundEffect (default true).
func WithGenerateSoundEffect(b bool) Option {
	return func(t *ElevenLabsTools) { t.enableGenerateSoundEffect = b }
}

// WithTextToSpeech toggles TextToSpeech (default true).
func WithTextToSpeech(b bool) Option { return func(t *ElevenLabsTools) { t.enableTextToSpeech = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(t *ElevenLabsTools) {
		t.enableGetVoices, t.enableGenerateSoundEffect, t.enableTextToSpeech = true, true, true
	}
}

// GetVoicesParams has no parameters.
type GetVoicesParams struct{}

// SoundEffectParams are the GenerateSoundEffect parameters.
type SoundEffectParams struct {
	Prompt          string  `json:"prompt" description:"Description of the sound effect" required:"true"`
	DurationSeconds float64 `json:"duration_seconds,omitempty" description:"Duration in seconds to generate audio from. Has to be between 0.5 and 22."`
}

// TextToSpeechParams are the TextToSpeech parameters.
type TextToSpeechParams struct {
	Prompt string `json:"prompt" description:"Text to generate audio from." required:"true"`
}

// NewElevenLabsTools creates the toolkit.
func NewElevenLabsTools(opts ...Option) *ElevenLabsTools {
	t := &ElevenLabsTools{
		apiKey:                    os.Getenv("ELEVEN_LABS_API_KEY"),
		baseURL:                   DefaultBaseURL,
		voiceID:                   "JBFqnCBsd6RMkjVDRZzb",
		modelID:                   "eleven_multilingual_v2",
		outputFormat:              "mp3_44100_64",
		enableGetVoices:           true,
		enableGenerateSoundEffect: true,
		enableTextToSpeech:        true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "elevenlabs_tools"
	t.Toolkit.Description = "Text-to-speech, sound effect generation and voice listing with ElevenLabs."
	t.Toolkit.Timeout = 2 * time.Minute
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.targetDirectory != "" {
		_ = os.MkdirAll(t.targetDirectory, 0o755)
	}
	if t.enableGetVoices {
		t.Toolkit.Register("GetVoices", "Get all the voices available. Returns a list of voices that have an ID, name and description.", t, t.GetVoices, GetVoicesParams{})
	}
	if t.enableGenerateSoundEffect {
		t.Toolkit.Register("GenerateSoundEffect", "Generate a sound effect from a text description.", t, t.GenerateSoundEffect, SoundEffectParams{})
	}
	if t.enableTextToSpeech {
		t.Toolkit.Register("TextToSpeech", "Convert text to speech.", t, t.TextToSpeech, TextToSpeechParams{})
	}
	return t
}

func (t *ElevenLabsTools) client() *restapi.Client {
	return restapi.New(t.baseURL, t.httpClient, map[string]string{"xi-api-key": t.apiKey})
}

// Audios returns the audio artifacts generated so far.
func (t *ElevenLabsTools) Audios() []Audio {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Audio(nil), t.audios...)
}

// GetVoices lists the available voices.
func (t *ElevenLabsTools) GetVoices(GetVoicesParams) (interface{}, error) {
	var resp struct {
		Voices []struct {
			VoiceID     string `json:"voice_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"voices"`
	}
	if err := t.client().JSON(context.Background(), http.MethodGet, "/v1/voices", nil, nil, &resp); err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	out := make([]map[string]string, 0, len(resp.Voices))
	for _, v := range resp.Voices {
		out = append(out, map[string]string{"id": v.VoiceID, "name": v.Name, "description": v.Description})
	}
	return restapi.ToJSON(out), nil
}

// GenerateSoundEffect generates a sound effect from a prompt.
func (t *ElevenLabsTools) GenerateSoundEffect(p SoundEffectParams) (interface{}, error) {
	body := map[string]interface{}{"text": p.Prompt}
	if p.DurationSeconds > 0 {
		body["duration_seconds"] = p.DurationSeconds
	}
	data, err := t.audioRequest("/v1/sound-generation", nil, body)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	a, err := t.processAudio(data)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return t.result("Sound effect generated successfully", a), nil
}

// TextToSpeech converts the prompt to speech with the configured voice.
func (t *ElevenLabsTools) TextToSpeech(p TextToSpeechParams) (interface{}, error) {
	q := url.Values{"output_format": {t.outputFormat}}
	data, err := t.audioRequest("/v1/text-to-speech/"+url.PathEscape(t.voiceID), q, map[string]interface{}{
		"text": p.Prompt, "model_id": t.modelID,
	})
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	a, err := t.processAudio(data)
	if err != nil {
		return fmt.Sprintf("Error: %v", err), nil
	}
	return t.result("Audio generated successfully", a), nil
}

func (t *ElevenLabsTools) audioRequest(path string, q url.Values, body interface{}) ([]byte, error) {
	c := t.client()
	c.Headers["Accept"] = "audio/*"
	return c.Do(context.Background(), http.MethodPost, path, q, body)
}

func (t *ElevenLabsTools) processAudio(data []byte) (Audio, error) {
	a := Audio{ID: uuid.NewString(), MimeType: "audio/mpeg", Content: data}
	if t.targetDirectory != "" {
		ext := "mp3"
		switch {
		case strings.HasPrefix(t.outputFormat, "pcm"):
			ext = "wav"
		case strings.HasPrefix(t.outputFormat, "ulaw"):
			ext = "ulaw"
		}
		a.Path = filepath.Join(t.targetDirectory, uuid.NewString()+"."+ext)
		if err := os.WriteFile(a.Path, data, 0o644); err != nil {
			return a, err
		}
	}
	t.mu.Lock()
	t.audios = append(t.audios, a)
	t.mu.Unlock()
	return a, nil
}

func (t *ElevenLabsTools) result(msg string, a Audio) string {
	out := map[string]interface{}{"content": msg, "audio_id": a.ID, "mime_type": a.MimeType, "size_bytes": len(a.Content)}
	if a.Path != "" {
		out["path"] = a.Path
	}
	b, _ := json.Marshal(out)
	return string(b)
}

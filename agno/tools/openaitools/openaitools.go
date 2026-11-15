// Package openaitools ports Agno's OpenAITools: Whisper transcription, image
// generation and text-to-speech over the OpenAI REST API.
package openaitools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/mediaout"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the OpenAI API root.
const DefaultBaseURL = "https://api.openai.com/v1"

// OpenAITools exposes OpenAI audio/image endpoints as agent tools.
type OpenAITools struct {
	toolkit.Toolkit
	apiKey             string
	baseURL            string
	httpClient         *http.Client
	transcriptionModel string
	ttsVoice           string
	ttsModel           string
	ttsFormat          string
	imageModel         string
	imageQuality       string
	imageSize          string
	imageStyle         string
	targetDirectory    string

	enableTranscription bool
	enableImage         bool
	enableSpeech        bool
}

// Option configures OpenAITools.
type Option func(*OpenAITools)

// WithAPIKey sets the API key (default OPENAI_API_KEY).
func WithAPIKey(k string) Option { return func(t *OpenAITools) { t.apiKey = k } }

// WithBaseURL overrides the API base URL.
func WithBaseURL(u string) Option { return func(t *OpenAITools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *OpenAITools) { t.httpClient = c } }

// WithTimeout sets the request timeout.
func WithTimeout(d time.Duration) Option { return func(t *OpenAITools) { t.Toolkit.Timeout = d } }

// WithTranscription enables/disables TranscribeAudio (default true).
func WithTranscription(on bool) Option { return func(t *OpenAITools) { t.enableTranscription = on } }

// WithImageGeneration enables/disables GenerateImage (default true).
func WithImageGeneration(on bool) Option { return func(t *OpenAITools) { t.enableImage = on } }

// WithSpeechGeneration enables/disables GenerateSpeech (default true).
func WithSpeechGeneration(on bool) Option { return func(t *OpenAITools) { t.enableSpeech = on } }

// WithAll enables every tool.
func WithAll() Option {
	return func(t *OpenAITools) { t.enableTranscription, t.enableImage, t.enableSpeech = true, true, true }
}

// WithTranscriptionModel sets the transcription model (default whisper-1).
func WithTranscriptionModel(m string) Option {
	return func(t *OpenAITools) { t.transcriptionModel = m }
}

// WithTextToSpeechVoice sets the TTS voice (default alloy).
func WithTextToSpeechVoice(v string) Option { return func(t *OpenAITools) { t.ttsVoice = v } }

// WithTextToSpeechModel sets the TTS model (default tts-1).
func WithTextToSpeechModel(m string) Option { return func(t *OpenAITools) { t.ttsModel = m } }

// WithTextToSpeechFormat sets the TTS format (default mp3).
func WithTextToSpeechFormat(f string) Option { return func(t *OpenAITools) { t.ttsFormat = f } }

// WithImageModel sets the image model (default gpt-image-2).
func WithImageModel(m string) Option { return func(t *OpenAITools) { t.imageModel = m } }

// WithImageQuality sets the image quality.
func WithImageQuality(q string) Option { return func(t *OpenAITools) { t.imageQuality = q } }

// WithImageSize sets the image size.
func WithImageSize(s string) Option { return func(t *OpenAITools) { t.imageSize = s } }

// WithImageStyle sets the image style.
func WithImageStyle(s string) Option { return func(t *OpenAITools) { t.imageStyle = s } }

// WithTargetDirectory sets where generated media is saved (default: OS temp dir).
func WithTargetDirectory(d string) Option { return func(t *OpenAITools) { t.targetDirectory = d } }

// TranscribeAudioParams are the parameters of TranscribeAudio.
type TranscribeAudioParams struct {
	AudioPath string `json:"audio_path" description:"Path to the audio file" required:"true"`
}

// GenerateImageParams are the parameters of GenerateImage.
type GenerateImageParams struct {
	Prompt string `json:"prompt" description:"The text prompt to generate the image from." required:"true"`
}

// GenerateSpeechParams are the parameters of GenerateSpeech.
type GenerateSpeechParams struct {
	TextInput string `json:"text_input" description:"The text to synthesize into speech." required:"true"`
}

// NewOpenAITools creates the toolkit.
func NewOpenAITools(opts ...Option) *OpenAITools {
	t := &OpenAITools{
		apiKey:              os.Getenv("OPENAI_API_KEY"),
		baseURL:             DefaultBaseURL,
		transcriptionModel:  "whisper-1",
		ttsVoice:            "alloy",
		ttsModel:            "tts-1",
		ttsFormat:           "mp3",
		imageModel:          "gpt-image-2",
		enableTranscription: true,
		enableImage:         true,
		enableSpeech:        true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "openai_tools"
	t.Toolkit.Description = "Transcribe audio, generate images and generate speech with the OpenAI API."
	t.Toolkit.Timeout = 2 * time.Minute
	for _, o := range opts {
		o(t)
	}
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableTranscription {
		t.Toolkit.Register("TranscribeAudio", "Transcribe audio file using OpenAI's Whisper API.", t, t.TranscribeAudio, TranscribeAudioParams{})
	}
	if t.enableImage {
		t.Toolkit.Register("GenerateImage", "Generate images based on a text prompt.", t, t.GenerateImage, GenerateImageParams{})
	}
	if t.enableSpeech {
		t.Toolkit.Register("GenerateSpeech", "Generate speech from text using OpenAI's Text-to-Speech API.", t, t.GenerateSpeech, GenerateSpeechParams{})
	}
	return t
}

func (t *OpenAITools) client() *restapi.Client {
	return restapi.New(t.baseURL, t.httpClient, map[string]string{"Authorization": "Bearer " + t.apiKey})
}

// TranscribeAudio transcribes a local audio file.
func (t *OpenAITools) TranscribeAudio(p TranscribeAudioParams) (interface{}, error) {
	if t.apiKey == "" {
		return "Failed to transcribe audio: OPENAI_API_KEY not set", nil
	}
	data, err := os.ReadFile(p.AudioPath)
	if err != nil {
		return fmt.Sprintf("Failed to transcribe audio: %v", err), nil
	}
	out, err := t.client().Multipart(context.Background(), http.MethodPost, "/audio/transcriptions",
		map[string]string{"model": t.transcriptionModel, "response_format": "text"},
		[]restapi.FilePart{{Field: "file", Filename: filepath.Base(p.AudioPath), Data: data}})
	if err != nil {
		return fmt.Sprintf("Failed to transcribe audio: %v", err), nil
	}
	return strings.TrimSpace(string(out)), nil
}

// GenerateImage generates an image and saves it to the target directory.
func (t *OpenAITools) GenerateImage(p GenerateImageParams) (interface{}, error) {
	if t.apiKey == "" {
		return "Failed to generate image: OPENAI_API_KEY not set", nil
	}
	body := map[string]interface{}{"model": t.imageModel, "prompt": p.Prompt}
	if t.imageSize != "" {
		body["size"] = t.imageSize
	}
	if t.imageQuality != "" {
		body["quality"] = t.imageQuality
	}
	if t.imageStyle != "" {
		body["style"] = t.imageStyle
	}
	if !strings.HasPrefix(t.imageModel, "gpt-image") {
		body["response_format"] = "b64_json"
	}
	var resp struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := t.client().JSON(context.Background(), http.MethodPost, "/images/generations", nil, body, &resp); err != nil {
		return fmt.Sprintf("Failed to generate image: %v", err), nil
	}
	if len(resp.Data) == 0 {
		return "Failed to generate image: No data received from API.", nil
	}
	if resp.Data[0].B64JSON == "" {
		return "Failed to generate image: No content received from API.", nil
	}
	img, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if err != nil {
		return fmt.Sprintf("Failed to generate image: %v", err), nil
	}
	id := mediaout.NewID()
	path, err := mediaout.Save(t.targetDirectory, id, "png", img)
	if err != nil {
		return fmt.Sprintf("Failed to generate image: %v", err), nil
	}
	return mediaout.Result{
		Content: "Image generated successfully.",
		Images:  []mediaout.Artifact{{ID: id, Path: path, MimeType: "image/png", OriginalPrompt: p.Prompt}},
	}.String(), nil
}

// GenerateSpeech synthesizes speech and saves it to the target directory.
func (t *OpenAITools) GenerateSpeech(p GenerateSpeechParams) (interface{}, error) {
	if t.apiKey == "" {
		return "Failed to generate speech: OPENAI_API_KEY not set", nil
	}
	audio, err := t.client().Do(context.Background(), http.MethodPost, "/audio/speech", nil, map[string]interface{}{
		"model": t.ttsModel, "voice": t.ttsVoice, "input": p.TextInput, "response_format": t.ttsFormat,
	})
	if err != nil {
		return fmt.Sprintf("Failed to generate speech: %v", err), nil
	}
	id := mediaout.NewID()
	path, err := mediaout.Save(t.targetDirectory, id, t.ttsFormat, audio)
	if err != nil {
		return fmt.Sprintf("Failed to generate speech: %v", err), nil
	}
	return mediaout.Result{
		Content: fmt.Sprintf("Speech generated successfully with ID: %s", id),
		Audios:  []mediaout.Artifact{{ID: id, Path: path, MimeType: "audio/" + t.ttsFormat, Format: t.ttsFormat}},
	}.String(), nil
}

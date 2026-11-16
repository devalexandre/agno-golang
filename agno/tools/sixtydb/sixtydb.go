// Package sixtydb ports Agno's SixtyDBTools: workspace voice discovery and
// HTTP text-to-speech through the 60db API. Audio is mono PCM16 WAV at 24 kHz.
package sixtydb

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devalexandre/agno-golang/agno/tools/internal/mediaout"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

const (
	// SampleRate is the only sample rate the toolkit accepts.
	SampleRate = 24000
	// MaxResponseBytes caps the response size (32 MiB).
	MaxResponseBytes = 32 * 1024 * 1024
	// DefaultBaseURL is the 60db API root.
	DefaultBaseURL = "https://api.60db.ai"
)

// SixtyDBTools uses workspace voices through 60db's HTTP API.
type SixtyDBTools struct {
	toolkit.Toolkit
	apiKey          string
	defaultVoiceID  string
	speed           float64
	baseURL         string
	targetDirectory string
	httpClient      *http.Client

	enableTextToSpeech bool
	enableGetVoices    bool
}

// Option configures SixtyDBTools.
type Option func(*SixtyDBTools)

// WithAPIKey sets the API key (default SIXTYDB_API_KEY).
func WithAPIKey(k string) Option { return func(t *SixtyDBTools) { t.apiKey = k } }

// WithDefaultVoiceID sets the voice used when TextToSpeech gets none.
func WithDefaultVoiceID(v string) Option { return func(t *SixtyDBTools) { t.defaultVoiceID = v } }

// WithSpeed sets the speaking speed, 0.5..2.0 (default 1.0).
func WithSpeed(s float64) Option { return func(t *SixtyDBTools) { t.speed = s } }

// WithBaseURL overrides the API base URL (https, or http on loopback).
func WithBaseURL(u string) Option { return func(t *SixtyDBTools) { t.baseURL = u } }

// WithTimeout sets the per-operation timeout (default 60s).
func WithTimeout(d time.Duration) Option { return func(t *SixtyDBTools) { t.Toolkit.Timeout = d } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *SixtyDBTools) { t.httpClient = c } }

// WithTargetDirectory saves generated WAV files there (default: OS temp dir).
func WithTargetDirectory(d string) Option { return func(t *SixtyDBTools) { t.targetDirectory = d } }

// WithTextToSpeech enables/disables TextToSpeech (default true).
func WithTextToSpeech(on bool) Option { return func(t *SixtyDBTools) { t.enableTextToSpeech = on } }

// WithGetVoices enables/disables GetVoices (default true).
func WithGetVoices(on bool) Option { return func(t *SixtyDBTools) { t.enableGetVoices = on } }

// WithAll enables every tool.
func WithAll() Option {
	return func(t *SixtyDBTools) { t.enableTextToSpeech, t.enableGetVoices = true, true }
}

// GetVoicesParams are the parameters of GetVoices.
type GetVoicesParams struct {
	Model string `json:"model,omitempty" description:"Voice catalog tier, quality or fast. Default quality."`
}

// TextToSpeechParams are the parameters of TextToSpeech.
type TextToSpeechParams struct {
	Text    string `json:"text" description:"Speech text, up to 5000 characters." required:"true"`
	VoiceID string `json:"voice_id,omitempty" description:"Workspace voice ID; otherwise use default_voice_id."`
}

// NewSixtyDBTools creates the toolkit and validates its configuration like Python.
func NewSixtyDBTools(opts ...Option) (*SixtyDBTools, error) {
	t := &SixtyDBTools{
		apiKey: os.Getenv("SIXTYDB_API_KEY"), speed: 1.0, baseURL: DefaultBaseURL,
		enableTextToSpeech: true, enableGetVoices: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "sixtydb_tools"
	t.Toolkit.Description = "Workspace voice discovery and text-to-speech with 60db."
	t.Toolkit.Timeout = 60 * time.Second
	for _, o := range opts {
		o(t)
	}
	if strings.TrimSpace(t.apiKey) == "" {
		return nil, errors.New("provide api_key or SIXTYDB_API_KEY")
	}
	if math.IsNaN(t.speed) || t.speed < 0.5 || t.speed > 2.0 {
		return nil, errors.New("speed must be finite and between 0.5 and 2.0")
	}
	u, err := url.Parse(t.baseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return nil, errors.New("base_url must be HTTPS, or HTTP on loopback, without credentials or query")
	}
	t.baseURL = strings.TrimRight(t.baseURL, "/")
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	// Never follow redirects (mirrors follow_redirects=False).
	client := *t.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	t.httpClient = &client

	if t.enableTextToSpeech {
		t.Toolkit.Register("TextToSpeech", "Convert text to a WAV audio artifact (mono PCM16, 24 kHz) using a workspace voice.", t, t.TextToSpeech, TextToSpeechParams{})
	}
	if t.enableGetVoices {
		t.Toolkit.Register("GetVoices", "List voices available in the authenticated workspace. Returns JSON voice objects, or a sanitized error object.", t, t.GetVoices, GetVoicesParams{})
	}
	return t, nil
}

func (t *SixtyDBTools) request(method, endpoint string, body interface{}) ([]byte, string, error) {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, t.baseURL+endpoint, reader)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, "", errors.New("network request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("60db request failed (HTTP %d)", resp.StatusCode)
	}
	meta := map[string]interface{}{}
	for key, header := range map[string]string{"sample_rate": "X-Sample-Rate", "channels": "X-Channels", "bit_depth": "X-Bit-Depth"} {
		if v := resp.Header.Get(header); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, "", errors.New("60db returned invalid HTTP audio metadata")
			}
			meta[key] = float64(n)
		}
	}
	if err := validateMetadata(meta); err != nil {
		return nil, "", err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, "", errors.New("network request failed")
	}
	if len(data) > MaxResponseBytes {
		return nil, "", errors.New("60db response exceeds 32 MiB")
	}
	ct, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return data, strings.ToLower(ct), nil
}

func (t *SixtyDBTools) logFailure(op string, err error) {
	detail := strings.ReplaceAll(err.Error(), t.apiKey, "<REDACTED>")
	if len(detail) > 500 {
		detail = detail[:500]
	}
	log.Printf("60db %s failed: %s", op, detail)
}

func validateMetadata(record map[string]interface{}) error {
	if err := validateAudioMetadata(record); err != nil {
		return err
	}
	if cfg, ok := record["audio_config"]; ok {
		m, ok := cfg.(map[string]interface{})
		if !ok {
			return errors.New("60db returned invalid audio configuration")
		}
		if _, nested := m["audio_config"]; nested {
			return errors.New("60db returned invalid audio configuration")
		}
		return validateAudioMetadata(m)
	}
	return nil
}

func validateAudioMetadata(r map[string]interface{}) error {
	if r["success"] == false || r["type"] == "error" {
		return errors.New("60db reported a synthesis error")
	}
	if e, ok := r["error"]; ok && e != nil && e != "" && e != false {
		return errors.New("60db reported a synthesis error")
	}
	for _, k := range []string{"encoding", "audio_encoding", "output_format"} {
		if v, ok := r[k]; ok {
			switch strings.ToLower(fmt.Sprint(v)) {
			case "linear16", "pcm", "pcm16", "wav":
			default:
				return errors.New("60db returned incompatible audio encoding")
			}
		}
	}
	for k, want := range map[string]float64{"sample_rate": SampleRate, "sample_rate_hertz": SampleRate, "channels": 1, "bit_depth": 16} {
		if v, ok := r[k]; ok {
			if f, isNum := v.(float64); !isNum || f != want {
				return errors.New("60db returned incompatible audio metadata")
			}
		}
	}
	return nil
}

// extractPCM returns raw PCM16 mono 24 kHz samples from a WAV or raw payload.
func extractPCM(audio []byte) ([]byte, error) {
	if bytes.HasPrefix(audio, []byte("RIFF")) {
		if len(audio) < 12 || int(binary.LittleEndian.Uint32(audio[4:8]))+8 != len(audio) || string(audio[8:12]) != "WAVE" {
			return nil, errors.New("60db returned invalid or truncated WAV audio")
		}
		var fmtOK bool
		var pcm []byte
		for off := 12; off+8 <= len(audio); {
			id := string(audio[off : off+4])
			size := int(binary.LittleEndian.Uint32(audio[off+4 : off+8]))
			start := off + 8
			if start+size > len(audio) {
				return nil, errors.New("60db returned truncated WAV audio")
			}
			chunk := audio[start : start+size]
			switch id {
			case "fmt ":
				if size < 16 {
					return nil, errors.New("60db returned invalid or truncated WAV audio")
				}
				format := binary.LittleEndian.Uint16(chunk[0:2])
				channels := binary.LittleEndian.Uint16(chunk[2:4])
				rate := binary.LittleEndian.Uint32(chunk[4:8])
				bits := binary.LittleEndian.Uint16(chunk[14:16])
				if format != 1 || channels != 1 || rate != SampleRate || bits != 16 {
					return nil, errors.New("60db WAV must be mono PCM16 at 24000 Hz")
				}
				fmtOK = true
			case "data":
				pcm = chunk
			}
			off = start + size + size%2
		}
		if !fmtOK || pcm == nil {
			return nil, errors.New("60db returned invalid or truncated WAV audio")
		}
		audio = pcm
	} else if bytes.HasPrefix(audio, []byte("ID3")) || bytes.HasPrefix(audio, []byte("OggS")) || bytes.HasPrefix(audio, []byte("fLaC")) {
		return nil, errors.New("60db returned compressed audio instead of PCM")
	}
	if len(audio) == 0 || len(audio)%2 != 0 {
		return nil, errors.New("60db returned empty or incomplete PCM16 audio")
	}
	return audio, nil
}

// pcmToWAV wraps mono PCM16 samples in a WAV container.
func pcmToWAV(pcm []byte) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(SampleRate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(SampleRate*2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}

// GetVoices lists workspace voices.
func (t *SixtyDBTools) GetVoices(p GetVoicesParams) (interface{}, error) {
	model := p.Model
	if model == "" {
		model = "quality"
	}
	if model != "quality" && model != "fast" {
		return `{"error": "model must be quality or fast"}`, nil
	}
	fail := func(err error) (interface{}, error) {
		t.logFailure("voice discovery", err)
		return `{"error": "60db voice discovery failed"}`, nil
	}
	body, ct, err := t.request(http.MethodGet, "/voices?"+url.Values{"model": {model}}.Encode(), nil)
	if err != nil {
		return fail(err)
	}
	if ct != "application/json" {
		return fail(errors.New("expected JSON voice catalog"))
	}
	var result struct {
		Success *bool                    `json:"success"`
		Data    []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Success == nil || !*result.Success || result.Data == nil {
		return fail(errors.New("invalid voice catalog"))
	}
	voices := make([]map[string]interface{}, 0, len(result.Data))
	for _, v := range result.Data {
		if _, ok := v["voice_id"].(string); !ok {
			return fail(errors.New("invalid workspace voice"))
		}
		out := map[string]interface{}{}
		for _, k := range []string{"voice_id", "name", "model", "labels", "description"} {
			out[k] = v[k]
		}
		voices = append(voices, out)
	}
	b, _ := json.Marshal(voices)
	return string(b), nil
}

// TextToSpeech synthesizes text to a WAV file.
func (t *SixtyDBTools) TextToSpeech(p TextToSpeechParams) (interface{}, error) {
	voice := p.VoiceID
	if voice == "" {
		voice = t.defaultVoiceID
	}
	if strings.TrimSpace(p.Text) == "" || utf8.RuneCountInString(p.Text) > 5000 {
		return "Error: text must contain 1 to 5000 characters", nil
	}
	if strings.TrimSpace(voice) == "" {
		return "Error: provide a workspace voice_id or default_voice_id", nil
	}
	fail := func(err error) (interface{}, error) {
		t.logFailure("speech generation", err)
		return "Error: 60db speech generation failed", nil
	}
	body, ct, err := t.request(http.MethodPost, "/tts-synthesize", map[string]interface{}{
		"text": p.Text, "voice_id": voice, "speed": t.speed, "timestamp_type": "NONE",
		"audio_config": map[string]interface{}{"audio_encoding": "LINEAR16", "sample_rate_hertz": SampleRate},
	})
	if err != nil {
		return fail(err)
	}
	if ct != "application/json" {
		return fail(errors.New("60db expected a JSON audio response"))
	}
	var record map[string]interface{}
	if err := json.Unmarshal(body, &record); err != nil {
		return fail(errors.New("60db returned an invalid response object"))
	}
	if err := validateMetadata(record); err != nil {
		return fail(err)
	}
	result := record
	if _, ok := record["audio_base64"]; !ok {
		r, ok := record["backendResponse"].(map[string]interface{})
		if !ok {
			return fail(errors.New("60db returned an invalid audio result"))
		}
		result = r
	}
	if err := validateMetadata(result); err != nil {
		return fail(err)
	}
	encoded, ok := result["audio_base64"].(string)
	if !ok {
		return fail(errors.New("60db audio_base64 must be text"))
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fail(err)
	}
	pcm, err := extractPCM(raw)
	if err != nil {
		return fail(err)
	}
	id := mediaout.NewID()
	path, err := mediaout.Save(t.targetDirectory, id, "wav", pcmToWAV(pcm))
	if err != nil {
		return fail(err)
	}
	return mediaout.Result{
		Content: "Audio generated and attached.",
		Audios: []mediaout.Artifact{{ID: id, Path: path, MimeType: "audio/wav", Format: "wav",
			SampleRate: SampleRate}},
	}.String(), nil
}

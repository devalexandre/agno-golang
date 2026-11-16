// Package aimlapi ports Agno's AIMLAPITools: image, video, speech and
// transcription models from many vendors behind the AI/ML API gateway.
package aimlapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/mediaout"
	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the AI/ML API root.
const DefaultBaseURL = "https://api.aimlapi.com"

const aimlapiHost = "api.aimlapi.com"

// attributionHeaders are only sent to the official host.
var attributionHeaders = map[string]string{
	"HTTP-Referer":         "https://github.com/agno-agi/agno",
	"X-Title":              "Agno",
	"X-AIMLAPI-Partner-ID": "part_VhLgeTWXXG9RwBOTptNQtcq0",
	"X-AIMLAPI-Source":     "agent/agno",
}

// SpeechFormats are the accepted speech output containers.
var SpeechFormats = []string{"mp3", "opus", "aac", "flac", "wav", "pcm"}

var inProgress = map[string]bool{"queued": true, "generating": true, "processing": true, "pending": true,
	"running": true, "in_progress": true, "active": true, "waiting": true}

var transientStatuses = map[int]bool{408: true, 425: true, 429: true, 500: true, 502: true, 503: true, 504: true}

const maxTransientRetries = 3

var versionSuffix = regexp.MustCompile(`/v\d+/?$`)

// AIMLAPITools exposes AI/ML API media endpoints.
type AIMLAPITools struct {
	toolkit.Toolkit
	apiKey     string
	baseURL    string
	httpClient *http.Client

	imageModel, imageSize, imageQuality             string
	videoModel, videoResolution, videoAspectRatio   string
	videoDuration                                   int
	videoPollInterval, videoTimeout                 time.Duration
	speechModel, speechVoice, speechFormat          string
	speechSpeed                                     float64
	transcriptionModel, transcriptionLanguage       string
	transcriptionPollInterval, transcriptionTimeout time.Duration
	baseDir                                         string
	restrictToBaseDir                               bool
	targetDirectory                                 string
	allowInsecureDownloads                          bool

	enableImage, enableVideo, enableSpeech, enableTranscribe bool
}

// Option configures AIMLAPITools.
type Option func(*AIMLAPITools)

// WithAPIKey sets the API key (default AIMLAPI_API_KEY).
func WithAPIKey(k string) Option { return func(t *AIMLAPITools) { t.apiKey = k } }

// WithBaseURL overrides the API root (a trailing /vN is stripped).
func WithBaseURL(u string) Option { return func(t *AIMLAPITools) { t.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(t *AIMLAPITools) { t.httpClient = c } }

// WithTimeout sets the per-request timeout (default 120s).
func WithTimeout(d time.Duration) Option { return func(t *AIMLAPITools) { t.Toolkit.Timeout = d } }

// WithGenerateImage enables/disables GenerateImage (default true).
func WithGenerateImage(on bool) Option { return func(t *AIMLAPITools) { t.enableImage = on } }

// WithGenerateVideo enables/disables GenerateVideo (default true).
func WithGenerateVideo(on bool) Option { return func(t *AIMLAPITools) { t.enableVideo = on } }

// WithGenerateSpeech enables/disables GenerateSpeech (default true).
func WithGenerateSpeech(on bool) Option { return func(t *AIMLAPITools) { t.enableSpeech = on } }

// WithTranscribeAudio enables/disables TranscribeAudio (default true).
func WithTranscribeAudio(on bool) Option { return func(t *AIMLAPITools) { t.enableTranscribe = on } }

// WithAll enables every tool.
func WithAll() Option {
	return func(t *AIMLAPITools) {
		t.enableImage, t.enableVideo, t.enableSpeech, t.enableTranscribe = true, true, true, true
	}
}

// WithImage sets the image model, size and quality (empty values are omitted).
func WithImage(model, size, quality string) Option {
	return func(t *AIMLAPITools) { t.imageModel, t.imageSize, t.imageQuality = model, size, quality }
}

// WithVideo sets the video model, duration, resolution and aspect ratio (zero values are omitted).
func WithVideo(model string, duration int, resolution, aspectRatio string) Option {
	return func(t *AIMLAPITools) {
		t.videoModel, t.videoDuration, t.videoResolution, t.videoAspectRatio = model, duration, resolution, aspectRatio
	}
}

// WithVideoPolling sets the video poll interval (default 5s) and timeout (default 900s).
func WithVideoPolling(interval, timeout time.Duration) Option {
	return func(t *AIMLAPITools) { t.videoPollInterval, t.videoTimeout = interval, timeout }
}

// WithSpeech sets the speech model, voice, format and speed (speed 0 = omitted).
func WithSpeech(model, voice, format string, speed float64) Option {
	return func(t *AIMLAPITools) {
		t.speechModel, t.speechVoice, t.speechFormat, t.speechSpeed = model, voice, format, speed
	}
}

// WithTranscription sets the transcription model and language hint.
func WithTranscription(model, language string) Option {
	return func(t *AIMLAPITools) { t.transcriptionModel, t.transcriptionLanguage = model, language }
}

// WithTranscriptionPolling sets the transcription poll interval (default 2s) and timeout (default 300s).
func WithTranscriptionPolling(interval, timeout time.Duration) Option {
	return func(t *AIMLAPITools) { t.transcriptionPollInterval, t.transcriptionTimeout = interval, timeout }
}

// WithBaseDir sets the directory local audio files are read from (default cwd).
func WithBaseDir(d string) Option { return func(t *AIMLAPITools) { t.baseDir = d } }

// WithRestrictToBaseDir refuses local paths outside base_dir (default true).
func WithRestrictToBaseDir(on bool) Option { return func(t *AIMLAPITools) { t.restrictToBaseDir = on } }

// WithTargetDirectory saves downloaded media there (default: OS temp dir).
func WithTargetDirectory(d string) Option { return func(t *AIMLAPITools) { t.targetDirectory = d } }

// withInsecureDownloads allows http asset URLs (tests only).
func withInsecureDownloads() Option { return func(t *AIMLAPITools) { t.allowInsecureDownloads = true } }

// PromptParams are the parameters of GenerateImage and GenerateVideo.
type PromptParams struct {
	Prompt string `json:"prompt" description:"What the image or video should show." required:"true"`
}

// GenerateSpeechParams are the parameters of GenerateSpeech.
type GenerateSpeechParams struct {
	TextInput string `json:"text_input" description:"The text to read aloud." required:"true"`
}

// TranscribeAudioParams are the parameters of TranscribeAudio.
type TranscribeAudioParams struct {
	AudioPath string `json:"audio_path" description:"Path to an audio file inside the toolkit's base directory, or an https URL of one." required:"true"`
}

// NewAIMLAPITools creates the toolkit. It errors when no API key is set or the
// speech format is invalid, like the Python constructor.
func NewAIMLAPITools(opts ...Option) (*AIMLAPITools, error) {
	cwd, _ := os.Getwd()
	t := &AIMLAPITools{
		apiKey: os.Getenv("AIMLAPI_API_KEY"), baseURL: DefaultBaseURL,
		imageModel: "openai/gpt-image-2", videoModel: "bytedance/seedance-2-5",
		videoPollInterval: 5 * time.Second, videoTimeout: 900 * time.Second,
		speechModel: "openai/tts-1", speechVoice: "alloy", speechFormat: "mp3",
		transcriptionModel: "deepgram/nova-3", transcriptionPollInterval: 2 * time.Second,
		transcriptionTimeout: 300 * time.Second, baseDir: cwd, restrictToBaseDir: true,
		enableImage: true, enableVideo: true, enableSpeech: true, enableTranscribe: true,
	}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "aimlapi_tools"
	t.Toolkit.Description = "Image, video, speech and transcription models through AI/ML API."
	t.Toolkit.Timeout = 120 * time.Second
	for _, o := range opts {
		o(t)
	}
	if t.apiKey == "" {
		return nil, errors.New("AIMLAPI_API_KEY not set. Please set the AIMLAPI_API_KEY environment variable")
	}
	valid := false
	for _, f := range SpeechFormats {
		valid = valid || f == t.speechFormat
	}
	if !valid {
		return nil, fmt.Errorf("speech_format must be one of %s, got %q", strings.Join(SpeechFormats, ", "), t.speechFormat)
	}
	t.baseURL = versionSuffix.ReplaceAllString(strings.TrimRight(t.baseURL, "/"), "")
	if t.httpClient == nil {
		t.httpClient = t.Toolkit.HTTPClient()
	}
	if t.enableImage {
		t.Toolkit.Register("GenerateImage", "Generate an image from a text prompt.", t, t.GenerateImage, PromptParams{})
	}
	if t.enableVideo {
		t.Toolkit.Register("GenerateVideo", "Generate a short video from a text prompt. Takes a minute or more.", t, t.GenerateVideo, PromptParams{})
	}
	if t.enableSpeech {
		t.Toolkit.Register("GenerateSpeech", "Turn text into spoken audio.", t, t.GenerateSpeech, GenerateSpeechParams{})
	}
	if t.enableTranscribe {
		t.Toolkit.Register("TranscribeAudio", "Transcribe an audio file to text.", t, t.TranscribeAudio, TranscribeAudioParams{})
	}
	return t, nil
}

func (t *AIMLAPITools) client() *restapi.Client {
	h := map[string]string{"Authorization": "Bearer " + t.apiKey}
	if u, err := url.Parse(t.baseURL); err == nil && u.Hostname() == aimlapiHost {
		for k, v := range attributionHeaders {
			h[k] = v
		}
	}
	return restapi.New(t.baseURL, t.httpClient, h)
}

// apiErr converts restapi errors into the gateway's message.
func apiErr(err error) error {
	var e *restapi.APIError
	if errors.As(err, &e) {
		msg := e.Body
		var body map[string]interface{}
		if json.Unmarshal([]byte(e.Body), &body) == nil {
			if m, ok := body["message"].(string); ok && m != "" {
				msg = m
			} else if m := errorText(body["error"]); m != "" {
				msg = m
			}
		}
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return &gatewayError{status: e.Status, msg: msg}
	}
	return err
}

type gatewayError struct {
	status int
	msg    string
}

func (e *gatewayError) Error() string {
	return fmt.Sprintf("AI/ML API returned HTTP %d: %s", e.status, e.msg)
}

func (t *AIMLAPITools) call(method, p string, q url.Values, body interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := t.client().JSON(context.Background(), method, p, q, body, &out); err != nil {
		return nil, apiErr(err)
	}
	if out == nil {
		return nil, errors.New("AI/ML API returned a non-object JSON body")
	}
	return out, nil
}

func (t *AIMLAPITools) download(raw string) ([]byte, string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && !(t.allowInsecureDownloads && u.Scheme == "http")) {
		return nil, "", errors.New("AI/ML API returned a non-HTTPS asset URL")
	}
	resp, err := t.httpClient.Get(raw) // public link: no key travels with it
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("asset download failed with HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, restapi.MaxResponseBytes))
	if err != nil {
		return nil, "", err
	}
	return data, assetMimeType(resp.Header.Get("Content-Type"), u.Path), nil
}

func (t *AIMLAPITools) poll(fetch func() (map[string]interface{}, error), interval, timeout time.Duration, what string) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)
	failures := 0
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("%s still running after %.0fs", what, timeout.Seconds())
		}
		if interval < remaining {
			remaining = interval
		}
		time.Sleep(remaining)
		job, err := fetch()
		if err != nil {
			var ge *gatewayError
			transient := !errors.As(err, &ge) || transientStatuses[ge.status]
			if !transient || failures >= maxTransientRetries {
				return nil, err
			}
			failures++
			continue
		}
		failures = 0
		if s, _ := job["status"].(string); !inProgress[s] {
			return job, nil
		}
	}
}

func (t *AIMLAPITools) save(data []byte, mimeType string) (string, string, error) {
	id := mediaout.NewID()
	p, err := mediaout.Save(t.targetDirectory, id, subtype(mimeType), data)
	return id, p, err
}

// GenerateImage generates an image.
func (t *AIMLAPITools) GenerateImage(p PromptParams) (interface{}, error) {
	body := map[string]interface{}{"model": t.imageModel, "prompt": p.Prompt}
	if t.imageSize != "" {
		body["size"] = t.imageSize
	}
	if t.imageQuality != "" {
		body["quality"] = t.imageQuality
	}
	payload, err := t.call(http.MethodPost, "/v1/images/generations", nil, body)
	if err != nil {
		return fmt.Sprintf("Failed to generate image: %v", err), nil
	}
	urls := assetURLs(payload["data"])
	if len(urls) == 0 {
		return "Failed to generate image: No image data received from API.", nil
	}
	res := mediaout.Result{Content: "Image generated successfully."}
	for _, u := range urls {
		data, mt, err := t.download(u)
		if err != nil {
			return fmt.Sprintf("Failed to generate image: %v", err), nil
		}
		id, path, err := t.save(data, mt)
		if err != nil {
			return fmt.Sprintf("Failed to generate image: %v", err), nil
		}
		res.Images = append(res.Images, mediaout.Artifact{ID: id, Path: path, URL: u, MimeType: mt, Format: subtype(mt), OriginalPrompt: p.Prompt})
	}
	return res.String(), nil
}

// GenerateVideo generates a short video.
func (t *AIMLAPITools) GenerateVideo(p PromptParams) (interface{}, error) {
	body := map[string]interface{}{"model": t.videoModel, "prompt": p.Prompt}
	if t.videoDuration != 0 {
		body["duration"] = t.videoDuration
	}
	if t.videoResolution != "" {
		body["resolution"] = t.videoResolution
	}
	if t.videoAspectRatio != "" {
		body["aspect_ratio"] = t.videoAspectRatio
	}
	job, err := t.call(http.MethodPost, "/v2/video/generations", nil, body)
	if err != nil {
		return fmt.Sprintf("Failed to generate video: %v", err), nil
	}
	jobID, _ := job["id"].(string)
	if jobID == "" {
		return "Failed to generate video: API did not return a generation id.", nil
	}
	if s, _ := job["status"].(string); inProgress[s] {
		job, err = t.poll(func() (map[string]interface{}, error) {
			return t.call(http.MethodGet, "/v2/video/generations", url.Values{"generation_id": {jobID}}, nil)
		}, t.videoPollInterval, t.videoTimeout, "video generation")
		if err != nil {
			return fmt.Sprintf("Failed to generate video: %v", err), nil
		}
	}
	if msg := jobFailure(job, "generate video"); msg != "" {
		return msg, nil
	}
	u := assetURL(job["video"])
	if u == "" {
		return "Failed to generate video: No video data received from API.", nil
	}
	data, mt, err := t.download(u)
	if err != nil {
		return fmt.Sprintf("Failed to generate video: %v", err), nil
	}
	id, path, err := t.save(data, mt)
	if err != nil {
		return fmt.Sprintf("Failed to generate video: %v", err), nil
	}
	return mediaout.Result{Content: "Video generated successfully.", Videos: []mediaout.Artifact{{
		ID: id, Path: path, URL: u, MimeType: mt, Format: subtype(mt), OriginalPrompt: p.Prompt}}}.String(), nil
}

// GenerateSpeech turns text into audio.
func (t *AIMLAPITools) GenerateSpeech(p GenerateSpeechParams) (interface{}, error) {
	body := map[string]interface{}{"model": t.speechModel, "text": p.TextInput, "response_format": t.speechFormat}
	if t.speechVoice != "" {
		body["voice"] = t.speechVoice
	}
	if t.speechSpeed != 0 {
		body["speed"] = t.speechSpeed
	}
	payload, err := t.call(http.MethodPost, "/v1/tts", nil, body)
	if err != nil {
		return fmt.Sprintf("Failed to generate speech: %v", err), nil
	}
	u := assetURL(payload["audio"])
	if u == "" {
		return "Failed to generate speech: No audio data received from API.", nil
	}
	data, mt, err := t.download(u)
	if err != nil {
		return fmt.Sprintf("Failed to generate speech: %v", err), nil
	}
	id, path, err := t.save(data, mt)
	if err != nil {
		return fmt.Sprintf("Failed to generate speech: %v", err), nil
	}
	return mediaout.Result{Content: fmt.Sprintf("Speech generated successfully with ID: %s", id),
		Audios: []mediaout.Artifact{{ID: id, Path: path, URL: u, MimeType: mt, Format: t.speechFormat}}}.String(), nil
}

func (t *AIMLAPITools) localAudio(p string) (string, error) {
	base, err := filepath.Abs(t.baseDir)
	if err != nil {
		return "", err
	}
	resolved := p
	if !filepath.IsAbs(p) {
		resolved = filepath.Join(base, p)
	}
	if r, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = r
	}
	if b, err := filepath.EvalSymlinks(base); err == nil {
		base = b
	}
	if t.restrictToBaseDir {
		rel, err := filepath.Rel(base, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("%s is outside the allowed directory %s", p, t.baseDir)
		}
	}
	if st, err := os.Stat(resolved); err != nil || !st.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a file", p)
	}
	return resolved, nil
}

// TranscribeAudio transcribes a local file or https URL.
func (t *AIMLAPITools) TranscribeAudio(p TranscribeAudioParams) (interface{}, error) {
	fields := map[string]string{"model": t.transcriptionModel}
	if t.transcriptionLanguage != "" {
		fields["language"] = t.transcriptionLanguage
	}
	var job map[string]interface{}
	var err error
	if strings.HasPrefix(p.AudioPath, "http://") || strings.HasPrefix(p.AudioPath, "https://") {
		body := map[string]interface{}{"url": p.AudioPath}
		for k, v := range fields {
			body[k] = v
		}
		job, err = t.call(http.MethodPost, "/v1/stt/create", nil, body)
	} else {
		var local string
		local, err = t.localAudio(p.AudioPath)
		if err == nil {
			var data []byte
			data, err = os.ReadFile(local)
			if err == nil {
				var raw []byte
				raw, err = t.client().Multipart(context.Background(), http.MethodPost, "/v1/stt/create", fields,
					[]restapi.FilePart{{Field: "audio", Filename: filepath.Base(local), Data: data}})
				if err != nil {
					err = apiErr(err)
				} else if jerr := json.Unmarshal(raw, &job); jerr != nil || job == nil {
					err = errors.New("AI/ML API returned a non-object JSON body")
				}
			}
		}
	}
	if err != nil {
		return fmt.Sprintf("Failed to transcribe audio: %v", err), nil
	}
	jobID, _ := job["generation_id"].(string)
	if jobID == "" {
		return "Failed to transcribe audio: API did not return a generation id.", nil
	}
	if s, _ := job["status"].(string); inProgress[s] {
		job, err = t.poll(func() (map[string]interface{}, error) {
			return t.call(http.MethodGet, "/v1/stt/"+url.PathEscape(jobID), nil, nil)
		}, t.transcriptionPollInterval, t.transcriptionTimeout, "transcription")
		if err != nil {
			return fmt.Sprintf("Failed to transcribe audio: %v", err), nil
		}
	}
	if msg := jobFailure(job, "transcribe audio"); msg != "" {
		return msg, nil
	}
	if tr, ok := transcript(job); ok {
		return tr, nil
	}
	return "Failed to transcribe audio: No transcript received from API.", nil
}

func jobFailure(job map[string]interface{}, what string) string {
	status, _ := job["status"].(string)
	switch status {
	case "completed":
		return ""
	case "error", "failed":
		msg := errorText(job["error"])
		if msg == "" {
			msg = "generation failed"
		}
		return fmt.Sprintf("Failed to %s: %s", what, msg)
	}
	return fmt.Sprintf("Failed to %s: job ended with status '%v'", what, job["status"])
}

func transcript(job map[string]interface{}) (string, bool) {
	result, ok := job["result"].(map[string]interface{})
	if !ok {
		return "", false
	}
	for _, k := range []string{"text", "transcript"} {
		if s, ok := result[k].(string); ok {
			return s, true
		}
	}
	results, _ := result["results"].(map[string]interface{})
	channels, _ := results["channels"].([]interface{})
	for _, ch := range channels {
		chm, _ := ch.(map[string]interface{})
		alts, _ := chm["alternatives"].([]interface{})
		for _, a := range alts {
			am, _ := a.(map[string]interface{})
			if s, ok := am["transcript"].(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

func errorText(e interface{}) string {
	switch v := e.(type) {
	case string:
		return v
	case map[string]interface{}:
		for _, k := range []string{"message", "detail", "name"} {
			if s, ok := v[k].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func assetURL(v interface{}) string {
	if arr, ok := v.([]interface{}); ok {
		if len(arr) == 0 {
			return ""
		}
		v = arr[0]
	}
	if m, ok := v.(map[string]interface{}); ok {
		v = m["url"]
	}
	s, _ := v.(string)
	return s
}

func assetURLs(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		if u := assetURL(item); u != "" {
			out = append(out, u)
		}
	}
	return out
}

func assetMimeType(contentType, urlPath string) string {
	declared := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if declared != "" && declared != "application/octet-stream" {
		return declared
	}
	if guessed := mime.TypeByExtension(path.Ext(urlPath)); guessed != "" {
		return strings.SplitN(guessed, ";", 2)[0]
	}
	if declared != "" {
		return declared
	}
	return "application/octet-stream"
}

func subtype(m string) string {
	sub := m
	if i := strings.Index(m, "/"); i >= 0 {
		sub = m[i+1:]
	}
	switch sub {
	case "mpeg":
		return "mp3"
	case "x-wav":
		return "wav"
	case "quicktime":
		return "mov"
	}
	return sub
}

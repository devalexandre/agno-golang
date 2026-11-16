package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
)

// YouTubeReader fetches a video's transcript and returns it as one document.
//
// Port of agno-python knowledge/reader/youtube_reader.py, which relies on
// youtube_transcript_api. That library's approach is reproduced here with no
// dependency: the watch page is fetched for the INNERTUBE_API_KEY, the
// innertube /youtubei/v1/player endpoint (ANDROID client) returns the caption
// tracks, and the chosen track's timedtext XML is parsed. For each preferred
// language a manually created track wins over an auto-generated one.
type YouTubeReader struct {
	BaseReader
	// Languages in order of preference (default ["en"]).
	Languages  []string
	HTTPClient *http.Client
	// BaseURL of YouTube (default https://www.youtube.com); overridable for tests.
	BaseURL string
	// ClientVersion of the innertube ANDROID client.
	ClientVersion string
}

// NewYouTubeReader creates a YouTube transcript reader.
func NewYouTubeReader() *YouTubeReader {
	return &YouTubeReader{
		BaseReader:    BaseReader{ID: "youtube", Extensions: []string{"youtube.com", "youtu.be"}},
		Languages:     []string{"en"},
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
		BaseURL:       "https://www.youtube.com",
		ClientVersion: "20.10.38",
	}
}

// ErrNoTranscript is returned when a video has no transcript in the requested languages.
var ErrNoTranscript = errors.New("youtube: no transcript found")

var youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// YouTubeVideoID extracts the video ID from a watch/short/embed/youtu.be URL
// or returns source itself when it already is an 11-character ID.
func YouTubeVideoID(source string) (string, error) {
	source = strings.TrimSpace(source)
	if youtubeIDPattern.MatchString(source) {
		return source, nil
	}
	u, err := url.Parse(source)
	if err == nil && u.Host != "" {
		host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
		if v := u.Query().Get("v"); v != "" {
			return v, nil
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if host == "youtu.be" && len(parts) > 0 && parts[0] != "" {
			return parts[0], nil
		}
		if len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live" || parts[0] == "v") {
			return parts[1], nil
		}
	}
	// Python's fallback: url.split("v=")[-1].split("&")[0]
	if i := strings.LastIndex(source, "v="); i >= 0 {
		return strings.SplitN(source[i+2:], "&", 2)[0], nil
	}
	return "", fmt.Errorf("youtube: cannot extract a video id from %q", source)
}

var (
	innertubeKeyPattern = regexp.MustCompile(`"INNERTUBE_API_KEY":\s*"([a-zA-Z0-9_-]+)"`)
	consentValuePattern = regexp.MustCompile(`name="v" value="(.*?)"`)
)

// Read fetches the transcript of the video at source (URL or video ID).
func (r *YouTubeReader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	videoID, err := YouTubeVideoID(source)
	if err != nil {
		return nil, err
	}
	segments, lang, err := r.FetchTranscript(ctx, videoID)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, s := range segments {
		b.WriteString(s.Text)
		b.WriteString(" ")
	}
	doc := document.NewDocument(strings.TrimSpace(b.String()))
	doc.ID = "youtube_" + videoID
	doc.Name = "youtube_" + videoID
	doc.Source = source
	doc.ContentType = "text/plain"
	doc.Metadata["video_url"] = source
	doc.Metadata["video_id"] = videoID
	doc.Metadata["language"] = lang
	return []*document.Document{doc}, nil
}

// TranscriptSegment is one timed caption line.
type TranscriptSegment struct {
	Text     string
	Start    float64
	Duration float64
}

type captionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}

// FetchTranscript returns the transcript segments of videoID and the language used.
func (r *YouTubeReader) FetchTranscript(ctx context.Context, videoID string) ([]TranscriptSegment, string, error) {
	base := strings.TrimRight(r.BaseURL, "/")
	if base == "" {
		base = "https://www.youtube.com"
	}
	client := r.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	watchURL := base + "/watch?v=" + url.QueryEscape(videoID)
	page, err := r.get(ctx, client, watchURL, "")
	if err != nil {
		return nil, "", err
	}
	if strings.Contains(page, `action="https://consent.youtube.com/s"`) {
		m := consentValuePattern.FindStringSubmatch(page)
		if m == nil {
			return nil, "", errors.New("youtube: failed to create consent cookie")
		}
		if page, err = r.get(ctx, client, watchURL, "CONSENT=YES+"+m[1]); err != nil {
			return nil, "", err
		}
	}
	km := innertubeKeyPattern.FindStringSubmatch(page)
	if km == nil {
		if strings.Contains(page, `class="g-recaptcha"`) {
			return nil, "", errors.New("youtube: request blocked (IP flagged, captcha required)")
		}
		return nil, "", errors.New("youtube: INNERTUBE_API_KEY not found in watch page")
	}

	payload := map[string]any{
		"context": map[string]any{"client": map[string]any{"clientName": "ANDROID", "clientVersion": r.clientVersion()}},
		"videoId": videoID,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/youtubei/v1/player?key="+url.QueryEscape(km[1]), bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("youtube: innertube player returned HTTP %d", resp.StatusCode)
	}
	var player struct {
		PlayabilityStatus struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"playabilityStatus"`
		Captions struct {
			Renderer struct {
				Tracks []captionTrack `json:"captionTracks"`
			} `json:"playerCaptionsTracklistRenderer"`
		} `json:"captions"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&player); err != nil {
		return nil, "", fmt.Errorf("youtube: decode player response: %w", err)
	}
	if st := player.PlayabilityStatus.Status; st != "" && st != "OK" {
		return nil, "", fmt.Errorf("youtube: video %s unplayable (%s): %s", videoID, st, player.PlayabilityStatus.Reason)
	}
	track := pickCaptionTrack(player.Captions.Renderer.Tracks, r.languages())
	if track == nil {
		return nil, "", fmt.Errorf("%w for video %s in %v", ErrNoTranscript, videoID, r.languages())
	}

	capURL, err := url.Parse(strings.ReplaceAll(track.BaseURL, "&fmt=srv3", ""))
	if err != nil {
		return nil, "", fmt.Errorf("youtube: bad caption url: %w", err)
	}
	baseURL, _ := url.Parse(base)
	if !capURL.IsAbs() {
		capURL = baseURL.ResolveReference(capURL)
	}
	if !strings.EqualFold(capURL.Hostname(), baseURL.Hostname()) {
		return nil, "", fmt.Errorf("youtube: caption url on unexpected host %q", capURL.Host)
	}
	raw, err := r.get(ctx, client, capURL.String(), "")
	if err != nil {
		return nil, "", err
	}
	segs, err := parseTimedText(raw)
	if err != nil {
		return nil, "", err
	}
	return segs, track.LanguageCode, nil
}

func (r *YouTubeReader) languages() []string {
	if len(r.Languages) == 0 {
		return []string{"en"}
	}
	return r.Languages
}

func (r *YouTubeReader) clientVersion() string {
	if r.ClientVersion == "" {
		return "20.10.38"
	}
	return r.ClientVersion
}

func pickCaptionTrack(tracks []captionTrack, languages []string) *captionTrack {
	for _, lang := range languages {
		for i := range tracks {
			if tracks[i].LanguageCode == lang && tracks[i].Kind != "asr" {
				return &tracks[i]
			}
		}
		for i := range tracks {
			if tracks[i].LanguageCode == lang && tracks[i].Kind == "asr" {
				return &tracks[i]
			}
		}
	}
	return nil
}

func (r *YouTubeReader) get(ctx context.Context, client *http.Client, u, cookie string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept-Language", "en-US")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", errors.New("youtube: too many requests (IP blocked)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtube: GET %s returned HTTP %d", u, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	return string(data), err
}

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

func parseTimedText(raw string) ([]TranscriptSegment, error) {
	var tt struct {
		Texts []struct {
			Start float64 `xml:"start,attr"`
			Dur   float64 `xml:"dur,attr"`
			Body  string  `xml:",innerxml"`
		} `xml:"text"`
	}
	if err := xml.Unmarshal([]byte(raw), &tt); err != nil {
		return nil, fmt.Errorf("youtube: parse transcript: %w", err)
	}
	var segs []TranscriptSegment
	for _, t := range tt.Texts {
		// innerxml keeps entity escapes; the text may itself contain escaped markup.
		text := stdhtml.UnescapeString(t.Body)
		text = stdhtml.UnescapeString(htmlTagPattern.ReplaceAllString(text, ""))
		if text = strings.TrimSpace(text); text == "" {
			continue
		}
		segs = append(segs, TranscriptSegment{Text: text, Start: t.Start, Duration: t.Dur})
	}
	return segs, nil
}

var _ Reader = (*YouTubeReader)(nil)

// Package mediaout renders the media a toolkit produced into a tool-visible
// result. It is the Go stand-in for Python's ToolResult(images=..., audios=...,
// videos=...): generated binaries are written to disk and referenced by path,
// while remote media is referenced by URL.
package mediaout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Artifact describes one generated media item.
type Artifact struct {
	ID             string `json:"id"`
	URL            string `json:"url,omitempty"`
	Path           string `json:"path,omitempty"`
	MimeType       string `json:"mime_type,omitempty"`
	Format         string `json:"format,omitempty"`
	SampleRate     int    `json:"sample_rate,omitempty"`
	OriginalPrompt string `json:"original_prompt,omitempty"`
	ETA            string `json:"eta,omitempty"`
}

// Result mirrors Python's ToolResult content + media lists.
type Result struct {
	Content string     `json:"content"`
	Images  []Artifact `json:"images,omitempty"`
	Videos  []Artifact `json:"videos,omitempty"`
	Audios  []Artifact `json:"audios,omitempty"`
}

// String renders the result as JSON. A result without media renders as its
// plain content, matching how Python surfaces ToolResult.content.
func (r Result) String() string {
	if len(r.Images) == 0 && len(r.Videos) == 0 && len(r.Audios) == 0 {
		return r.Content
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return r.Content
	}
	return string(data)
}

// Text returns a plain content-only result string.
func Text(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// NewID returns a random media id.
func NewID() string { return uuid.NewString() }

// DefaultDir is where binaries are saved when a toolkit has no target directory.
func DefaultDir() string { return filepath.Join(os.TempDir(), "agno-media") }

// Save writes data to dir (DefaultDir when empty) as <id>.<ext> and returns the path.
func Save(dir, id, ext string, data []byte) (string, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext = strings.TrimPrefix(ext, ".")
	name := id
	if ext != "" {
		name += "." + ext
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// ExtForMime returns a file extension for a media type.
func ExtForMime(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	sub := mime
	if i := strings.Index(mime, "/"); i >= 0 {
		sub = mime[i+1:]
	}
	switch sub {
	case "mpeg":
		return "mp3"
	case "x-wav", "wave":
		return "wav"
	case "quicktime":
		return "mov"
	case "jpeg":
		return "jpg"
	case "octet-stream", "":
		return "bin"
	}
	return sub
}

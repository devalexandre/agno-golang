package models

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pngPixel is a minimal valid PNG, used so mime sniffing has something real to detect.
var pngPixel = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89,
}

func TestImageResolveInlineData(t *testing.T) {
	img := Image{Data: pngPixel}

	data, mime, err := img.Resolve(false)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if string(data) != string(pngPixel) {
		t.Errorf("Resolve() returned %d bytes, want %d", len(data), len(pngPixel))
	}
	if mime != "image/png" {
		t.Errorf("Resolve() mime = %q, want image/png (sniffed)", mime)
	}
}

func TestImageResolveDataURL(t *testing.T) {
	img := Image{URL: EncodeDataURL(pngPixel, "image/png")}

	data, mime, err := img.Resolve(false)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if string(data) != string(pngPixel) {
		t.Error("Resolve() did not decode the data URL payload")
	}
	if mime != "image/png" {
		t.Errorf("Resolve() mime = %q, want image/png", mime)
	}
}

// A remote URL must not be fetched unless the caller asks for it: providers that
// accept URLs should pass them through untouched.
func TestImageResolveRemoteNotFetchedByDefault(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngPixel)
	}))
	defer srv.Close()

	img := Image{URL: srv.URL + "/cat.png"}

	data, _, err := img.Resolve(false)
	if err != nil {
		t.Fatalf("Resolve(false) error = %v", err)
	}
	if len(data) != 0 {
		t.Error("Resolve(false) fetched the URL; it should have been left alone")
	}
	if hits != 0 {
		t.Errorf("server was hit %d times with fetchRemote=false", hits)
	}

	data, mime, err := img.Resolve(true)
	if err != nil {
		t.Fatalf("Resolve(true) error = %v", err)
	}
	if string(data) != string(pngPixel) {
		t.Error("Resolve(true) did not fetch the remote image")
	}
	if mime != "image/png" {
		t.Errorf("Resolve(true) mime = %q, want image/png", mime)
	}
}

func TestImageResolveEmpty(t *testing.T) {
	if _, _, err := (Image{ID: "x"}).Resolve(true); err == nil {
		t.Error("Resolve() on an image with neither Data nor URL should fail")
	}
}

func TestImageDataURL(t *testing.T) {
	// Inline bytes become a data URL.
	got, err := (Image{Data: pngPixel, MimeType: "image/png"}).DataURL()
	if err != nil {
		t.Fatalf("DataURL() error = %v", err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngPixel)
	if got != want {
		t.Errorf("DataURL() = %q, want %q", got, want)
	}

	// A remote URL passes through untouched.
	got, err = (Image{URL: "https://example.com/a.png"}).DataURL()
	if err != nil {
		t.Fatalf("DataURL() error = %v", err)
	}
	if got != "https://example.com/a.png" {
		t.Errorf("DataURL() = %q, want the URL unchanged", got)
	}
}

func TestAudioFormat(t *testing.T) {
	tests := []struct {
		name  string
		audio Audio
		want  string
	}{
		{"explicit format wins", Audio{Format: "mp3", MimeType: "audio/wav"}, "mp3"},
		{"mpeg normalizes to mp3", Audio{MimeType: "audio/mpeg"}, "mp3"},
		{"x-wav normalizes to wav", Audio{MimeType: "audio/x-wav"}, "wav"},
		{"plain wav", Audio{MimeType: "audio/wav"}, "wav"},
		{"unknown falls back to wav", Audio{}, "wav"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.audio.AudioFormat(); got != tt.want {
				t.Errorf("AudioFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFileFilename(t *testing.T) {
	if got := (File{Name: "report.pdf"}).Filename(); got != "report.pdf" {
		t.Errorf("Filename() = %q, want report.pdf", got)
	}
	if got := (File{MimeType: "application/pdf"}).Filename(); got != "file.pdf" {
		t.Errorf("Filename() = %q, want file.pdf (derived)", got)
	}
}

func TestMessageHasMedia(t *testing.T) {
	if (Message{Content: "hi"}).HasMedia() {
		t.Error("HasMedia() = true for a text-only message")
	}
	if !(Message{Images: []Image{{URL: "x"}}}).HasMedia() {
		t.Error("HasMedia() = false for a message carrying an image")
	}
}

func TestDecodeDataURL(t *testing.T) {
	data, mime, ok := decodeDataURL(EncodeDataURL([]byte("hello"), "text/plain"))
	if !ok {
		t.Fatal("decodeDataURL() failed on a valid data URL")
	}
	if string(data) != "hello" || mime != "text/plain" {
		t.Errorf("decodeDataURL() = (%q, %q), want (hello, text/plain)", data, mime)
	}

	if _, _, ok := decodeDataURL("https://example.com/a.png"); ok {
		t.Error("decodeDataURL() accepted a plain URL")
	}
	if _, _, ok := decodeDataURL("data:image/png;base64,!!!not-base64!!!"); ok {
		t.Error("decodeDataURL() accepted an invalid base64 payload")
	}
}

func TestCapabilitiesAndUnsupportedMedia(t *testing.T) {
	if !CapabilitiesFor("google").Video {
		t.Error("gemini should report video support")
	}
	if CapabilitiesFor("anthropic").Audio {
		t.Error("anthropic should not report audio support")
	}
	if CapabilitiesFor("does-not-exist").Vision {
		t.Error("unknown providers must default to text-only")
	}

	msg := Message{
		Images: []Image{{URL: "x"}},
		Audio:  []Audio{{URL: "y"}},
		Videos: []Video{{URL: "z"}},
	}

	// Gemini takes all three.
	if got := UnsupportedMedia("google", msg); len(got) != 0 {
		t.Errorf("UnsupportedMedia(google) = %v, want none", got)
	}

	// Anthropic takes images only.
	got := strings.Join(UnsupportedMedia("anthropic", msg), ",")
	if got != "audio,video" {
		t.Errorf("UnsupportedMedia(anthropic) = %q, want audio,video", got)
	}

	// Cohere is text-only.
	got = strings.Join(UnsupportedMedia("cohere", msg), ",")
	if got != "images,audio,video" {
		t.Errorf("UnsupportedMedia(cohere) = %q, want images,audio,video", got)
	}
}

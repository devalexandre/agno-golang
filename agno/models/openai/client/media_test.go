package client

import (
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestBuildUserMessageTextOnly(t *testing.T) {
	msg := buildUserMessage(models.Message{Role: models.TypeUserRole, Content: "hello"})

	if msg.OfUser == nil {
		t.Fatal("expected a user message")
	}
	if got := msg.OfUser.Content.OfString.Value; got != "hello" {
		t.Errorf("content = %q, want hello", got)
	}
	if len(msg.OfUser.Content.OfArrayOfContentParts) != 0 {
		t.Error("text-only messages must stay a plain string, not content parts")
	}
}

func TestBuildUserMessageWithImageURL(t *testing.T) {
	msg := buildUserMessage(models.Message{
		Role:    models.TypeUserRole,
		Content: "what is this?",
		Images:  []models.Image{{URL: "https://example.com/cat.png", Detail: "high"}},
	})

	parts := msg.OfUser.Content.OfArrayOfContentParts
	if len(parts) != 2 {
		t.Fatalf("got %d content parts, want 2 (text + image)", len(parts))
	}
	if parts[0].OfText == nil || parts[0].OfText.Text != "what is this?" {
		t.Error("first part should carry the prompt text")
	}
	if parts[1].OfImageURL == nil {
		t.Fatal("second part should be an image")
	}
	if got := parts[1].OfImageURL.ImageURL.URL; got != "https://example.com/cat.png" {
		t.Errorf("image URL = %q, want it passed through unchanged", got)
	}
	if got := parts[1].OfImageURL.ImageURL.Detail; got != "high" {
		t.Errorf("detail = %q, want high", got)
	}
}

func TestBuildUserMessageWithInlineImage(t *testing.T) {
	msg := buildUserMessage(models.Message{
		Role:   models.TypeUserRole,
		Images: []models.Image{{Data: []byte("fake-bytes"), MimeType: "image/jpeg"}},
	})

	parts := msg.OfUser.Content.OfArrayOfContentParts
	if len(parts) != 1 {
		t.Fatalf("got %d content parts, want 1 (image only, no text)", len(parts))
	}
	url := parts[0].OfImageURL.ImageURL.URL
	if !strings.HasPrefix(url, "data:image/jpeg;base64,") {
		t.Errorf("inline image should become a data URL, got %q", url)
	}
}

func TestBuildUserMessageWithAudioAndFile(t *testing.T) {
	msg := buildUserMessage(models.Message{
		Role:    models.TypeUserRole,
		Content: "transcribe",
		Audio:   []models.Audio{{Data: []byte("RIFFfake"), MimeType: "audio/wav"}},
		Files:   []models.File{{Data: []byte("%PDF-1.4"), Name: "doc.pdf", MimeType: "application/pdf"}},
	})

	parts := msg.OfUser.Content.OfArrayOfContentParts
	if len(parts) != 3 {
		t.Fatalf("got %d content parts, want 3 (text + audio + file)", len(parts))
	}

	if parts[1].OfInputAudio == nil {
		t.Fatal("expected an input_audio part")
	}
	if got := parts[1].OfInputAudio.InputAudio.Format; got != "wav" {
		t.Errorf("audio format = %q, want wav", got)
	}
	if parts[1].OfInputAudio.InputAudio.Data == "" {
		t.Error("audio payload should be base64 encoded, not empty")
	}

	if parts[2].OfFile == nil {
		t.Fatal("expected a file part")
	}
	if got := parts[2].OfFile.File.Filename.Value; got != "doc.pdf" {
		t.Errorf("filename = %q, want doc.pdf", got)
	}
	if !strings.HasPrefix(parts[2].OfFile.File.FileData.Value, "data:application/pdf;base64,") {
		t.Error("file_data should be a data URL")
	}
}

// Video has no representation in the Chat Completions API: it must be dropped,
// not sent as something else and not made to fail the request.
func TestBuildUserMessageDropsVideo(t *testing.T) {
	msg := buildUserMessage(models.Message{
		Role:    models.TypeUserRole,
		Content: "describe",
		Videos:  []models.Video{{URL: "https://example.com/clip.mp4"}},
	})

	parts := msg.OfUser.Content.OfArrayOfContentParts
	if len(parts) != 1 || parts[0].OfText == nil {
		t.Fatalf("got %d parts, want only the text part (video dropped)", len(parts))
	}
}

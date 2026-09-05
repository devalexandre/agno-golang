package gemini

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestToContentsTextOnly(t *testing.T) {
	contents := toContents([]models.Message{
		{Role: models.TypeUserRole, Content: "hello"},
	})

	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	if len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != "hello" {
		t.Error("text-only message should produce a single text part")
	}
}

func TestToContentsWithInlineImage(t *testing.T) {
	contents := toContents([]models.Message{{
		Role:    models.TypeUserRole,
		Content: "what is this?",
		Images:  []models.Image{{Data: []byte("bytes"), MimeType: "image/png"}},
	}})

	parts := contents[0].Parts
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2 (text + inline image)", len(parts))
	}
	if parts[0].Text != "what is this?" {
		t.Error("first part should be the prompt text")
	}
	if parts[1].InlineData == nil {
		t.Fatal("image with bytes should become an InlineData blob")
	}
	if parts[1].InlineData.MIMEType != "image/png" {
		t.Errorf("mime = %q, want image/png", parts[1].InlineData.MIMEType)
	}
	if string(parts[1].InlineData.Data) != "bytes" {
		t.Error("blob should carry the raw bytes")
	}
}

func TestToContentsWithRemoteURI(t *testing.T) {
	contents := toContents([]models.Message{{
		Role:   models.TypeUserRole,
		Videos: []models.Video{{URL: "gs://bucket/clip.mp4", MimeType: "video/mp4"}},
	}})

	parts := contents[0].Parts
	if len(parts) != 1 {
		t.Fatalf("got %d parts, want 1 (video only)", len(parts))
	}
	if parts[0].FileData == nil {
		t.Fatal("remote media should become a FileData reference, not a blob")
	}
	if parts[0].FileData.FileURI != "gs://bucket/clip.mp4" {
		t.Errorf("uri = %q, want the URL unchanged", parts[0].FileData.FileURI)
	}
}

// Gemini accepts every media kind; a data URL must be decoded into a blob rather
// than passed on as a URI.
func TestToContentsDecodesDataURL(t *testing.T) {
	contents := toContents([]models.Message{{
		Role:   models.TypeUserRole,
		Images: []models.Image{{URL: models.EncodeDataURL([]byte("png-bytes"), "image/png")}},
	}})

	parts := contents[0].Parts
	if parts[0].InlineData == nil {
		t.Fatal("data URL should be decoded into an InlineData blob")
	}
	if string(parts[0].InlineData.Data) != "png-bytes" {
		t.Errorf("blob data = %q, want the decoded payload", parts[0].InlineData.Data)
	}
}

func TestToContentsAllMediaKinds(t *testing.T) {
	contents := toContents([]models.Message{{
		Role:    models.TypeUserRole,
		Content: "analyze",
		Images:  []models.Image{{Data: []byte("i")}},
		Audio:   []models.Audio{{Data: []byte("a")}},
		Videos:  []models.Video{{Data: []byte("v")}},
		Files:   []models.File{{Data: []byte("f")}},
	}})

	if got := len(contents[0].Parts); got != 5 {
		t.Errorf("got %d parts, want 5 (text + 4 media kinds)", got)
	}
}

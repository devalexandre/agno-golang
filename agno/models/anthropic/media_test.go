package anthropic

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestUserBlocksTextOnly(t *testing.T) {
	blocks := userBlocks(models.Message{Role: models.TypeUserRole, Content: "hello"})

	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if blocks[0].OfText == nil || blocks[0].OfText.Text != "hello" {
		t.Error("expected a single text block carrying the prompt")
	}
}

func TestUserBlocksWithInlineImage(t *testing.T) {
	blocks := userBlocks(models.Message{
		Role:    models.TypeUserRole,
		Content: "what is this?",
		Images:  []models.Image{{Data: []byte("bytes"), MimeType: "image/png"}},
	})

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2 (text + image)", len(blocks))
	}
	if blocks[1].OfImage == nil {
		t.Fatal("second block should be an image")
	}
	src := blocks[1].OfImage.Source.OfBase64
	if src == nil {
		t.Fatal("inline bytes should produce a base64 image source")
	}
	if string(src.MediaType) != "image/png" {
		t.Errorf("media type = %q, want image/png", src.MediaType)
	}
	if src.Data == "" {
		t.Error("base64 payload should not be empty")
	}
}

func TestUserBlocksWithImageURL(t *testing.T) {
	blocks := userBlocks(models.Message{
		Role:   models.TypeUserRole,
		Images: []models.Image{{URL: "https://example.com/cat.png"}},
	})

	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 (image only)", len(blocks))
	}
	src := blocks[0].OfImage.Source.OfURL
	if src == nil {
		t.Fatal("a remote URL should produce a URL image source, not base64")
	}
	if src.URL != "https://example.com/cat.png" {
		t.Errorf("url = %q, want it passed through unchanged", src.URL)
	}
}

func TestUserBlocksDecodesDataURL(t *testing.T) {
	blocks := userBlocks(models.Message{
		Role:   models.TypeUserRole,
		Images: []models.Image{{URL: models.EncodeDataURL([]byte("png"), "image/png")}},
	})

	if blocks[0].OfImage.Source.OfBase64 == nil {
		t.Fatal("a data URL must be sent as a base64 source, not as a URL source")
	}
}

func TestUserBlocksWithPDF(t *testing.T) {
	blocks := userBlocks(models.Message{
		Role:  models.TypeUserRole,
		Files: []models.File{{Data: []byte("%PDF-1.4"), MimeType: "application/pdf"}},
	})

	if len(blocks) != 1 || blocks[0].OfDocument == nil {
		t.Fatal("expected a document block")
	}
	if blocks[0].OfDocument.Source.OfBase64 == nil {
		t.Error("inline PDF should become a base64 document source")
	}
}

// The Messages API has no audio or video representation: they must be dropped.
func TestUserBlocksDropsAudioAndVideo(t *testing.T) {
	blocks := userBlocks(models.Message{
		Role:    models.TypeUserRole,
		Content: "listen",
		Audio:   []models.Audio{{Data: []byte("wav")}},
		Videos:  []models.Video{{URL: "https://example.com/clip.mp4"}},
	})

	if len(blocks) != 1 || blocks[0].OfText == nil {
		t.Fatalf("got %d blocks, want only the text block", len(blocks))
	}
}

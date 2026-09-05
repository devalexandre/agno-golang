package agent

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

func TestAttachRunMediaToLastUserMessage(t *testing.T) {
	messages := []models.Message{
		{Role: models.TypeSystemRole, Content: "you are helpful"},
		{Role: models.TypeUserRole, Content: "first turn"},
		{Role: models.TypeAssistantRole, Content: "sure"},
		{Role: models.TypeUserRole, Content: "what is in this image?"},
	}

	got := attachRunMedia(messages,
		[]Image{{URL: "https://example.com/cat.png", MimeType: "image/png"}},
		nil, nil, nil,
	)

	// Media belongs on the current turn, not an earlier one.
	if len(got[3].Images) != 1 {
		t.Fatalf("last user message has %d images, want 1", len(got[3].Images))
	}
	if got[3].Images[0].URL != "https://example.com/cat.png" {
		t.Errorf("image URL = %q, want it carried through", got[3].Images[0].URL)
	}
	if len(got[1].Images) != 0 {
		t.Error("media leaked onto an earlier user message")
	}
	if len(got[0].Images) != 0 {
		t.Error("media leaked onto the system message")
	}
}

func TestAttachRunMediaAllKinds(t *testing.T) {
	messages := []models.Message{{Role: models.TypeUserRole, Content: "analyze"}}

	got := attachRunMedia(messages,
		[]Image{{Data: []byte("i")}},
		[]Audio{{Data: []byte("a")}},
		[]Video{{Data: []byte("v")}},
		[]File{{Data: []byte("f"), Name: "doc.pdf"}},
	)

	m := got[0]
	if len(m.Images) != 1 || len(m.Audio) != 1 || len(m.Videos) != 1 || len(m.Files) != 1 {
		t.Fatalf("media not fully attached: images=%d audio=%d videos=%d files=%d",
			len(m.Images), len(m.Audio), len(m.Videos), len(m.Files))
	}
	if m.Files[0].Name != "doc.pdf" {
		t.Errorf("file name = %q, want doc.pdf", m.Files[0].Name)
	}
	if !m.HasMedia() {
		t.Error("HasMedia() = false after attaching media")
	}
}

func TestAttachRunMediaNoMediaIsNoop(t *testing.T) {
	messages := []models.Message{{Role: models.TypeUserRole, Content: "hello"}}

	got := attachRunMedia(messages, nil, nil, nil, nil)

	if got[0].HasMedia() {
		t.Error("a run without media must leave the message untouched")
	}
}

// Without a user message there is nowhere to attach media; it must not panic or
// land on a system message.
func TestAttachRunMediaWithoutUserMessage(t *testing.T) {
	messages := []models.Message{{Role: models.TypeSystemRole, Content: "system only"}}

	got := attachRunMedia(messages, []Image{{URL: "x"}}, nil, nil, nil)

	if len(got) != 1 || got[0].HasMedia() {
		t.Error("media must not be attached when there is no user message")
	}
}

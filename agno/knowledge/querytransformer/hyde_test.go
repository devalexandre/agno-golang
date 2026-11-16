package querytransformer

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/models"
)

type fakeModel struct {
	reply    string
	err      error
	received []models.Message
}

func (f *fakeModel) Invoke(_ context.Context, messages []models.Message, _ ...models.Option) (*models.MessageResponse, error) {
	f.received = messages
	if f.err != nil {
		return nil, f.err
	}
	return &models.MessageResponse{Role: models.TypeAssistantRole, Content: f.reply}, nil
}
func (f *fakeModel) AInvoke(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (f *fakeModel) InvokeStream(context.Context, []models.Message, ...models.Option) error {
	return nil
}
func (f *fakeModel) AInvokeStream(context.Context, []models.Message, ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return nil, nil
}
func (f *fakeModel) GetID() string { return "fake" }

func TestHyDEReplacesQuery(t *testing.T) {
	m := &fakeModel{reply: "  Paris is the capital of France.  "}
	h := NewHyDE(m)
	got, err := h.Transform(context.Background(), "capital of France?")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Paris is the capital of France." {
		t.Fatalf("got %q", got)
	}
	if len(m.received) != 1 || m.received[0].Role != models.TypeUserRole ||
		!strings.Contains(m.received[0].Content, "Question: capital of France?") {
		t.Fatalf("unexpected prompt: %+v", m.received)
	}

	h.IncludeQuery = true
	got, _ = h.Transform(context.Background(), "capital of France?")
	if got != "capital of France?\n\nParis is the capital of France." {
		t.Fatalf("include query: got %q", got)
	}
}

func TestHyDEFallbacks(t *testing.T) {
	ctx := context.Background()
	if got, err := NewHyDE(&fakeModel{err: errors.New("down")}).Transform(ctx, "q"); err != nil || got != "q" {
		t.Fatalf("model failure should return query: %q %v", got, err)
	}
	if got, _ := NewHyDE(&fakeModel{reply: "   "}).Transform(ctx, "q"); got != "q" {
		t.Fatalf("empty reply should return query: %q", got)
	}
	if _, err := NewHyDE(nil).Transform(ctx, "q"); !errors.Is(err, ErrNoModel) {
		t.Fatalf("expected ErrNoModel, got %v", err)
	}
	h := NewHyDE(&fakeModel{reply: "x"})
	h.Prompt = "no placeholder {json}"
	if _, err := h.Transform(ctx, "q"); !errors.Is(err, ErrInvalidPrompt) {
		t.Fatalf("expected ErrInvalidPrompt, got %v", err)
	}
	h.Prompt = `{"example": 1} {query}`
	if _, err := h.Transform(ctx, "q"); err != nil {
		t.Fatalf("braces in prompt: %v", err)
	}
}

func TestHyDETruncate(t *testing.T) {
	h := &HyDE{MaxCharacters: 20}
	if got := h.truncate("First sentence. Second sentence here."); got != "First sentence." {
		t.Fatalf("sentence break: %q", got)
	}
	if got := h.truncate("alpha beta gamma delta epsilon"); got != "alpha beta gamma" {
		t.Fatalf("word break: %q", got)
	}
	if got := h.truncate("short"); got != "short" {
		t.Fatalf("short: %q", got)
	}
}

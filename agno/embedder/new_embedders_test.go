package embedder

import "testing"

// All new embedders must satisfy the Embedder interface and report sane defaults.
func TestNewEmbeddersImplementInterface(t *testing.T) {
	embedders := map[string]Embedder{
		"openai_like": NewOpenAILikeEmbedder("https://example.com/v1", "model-x", "k", 512),
		"mistral":     NewMistralEmbedder("k", "", 0),
		"jina":        NewJinaEmbedder("k", "", 0),
		"together":    NewTogetherEmbedder("k", "", 0),
		"nebius":      NewNebiusEmbedder("k", "", 0),
		"fireworks":   NewFireworksEmbedder("k", "", 0),
		"huggingface": NewHuggingFaceEmbedder("k", "", 0),
		"vllm":        NewVLLMEmbedder("", "", "k", 0),
		"google":      NewGoogleEmbedder("k", "", 0),
		"cohere":      NewCohereEmbedder("k", "", 0),
		"voyageai":    NewVoyageEmbedder("k", "", 0),
		"azure":       NewAzureOpenAIEmbedder("k", "https://r.openai.azure.com", "deploy", "", 0),
	}

	for name, e := range embedders {
		if e.GetID() == "" {
			t.Errorf("%s: empty ID", name)
		}
		if e.GetDimensions() <= 0 {
			t.Errorf("%s: non-positive dimensions %d", name, e.GetDimensions())
		}
	}
}

func TestEmbedderDefaults(t *testing.T) {
	cases := []struct {
		name string
		e    Embedder
		id   string
		dims int
	}{
		{"mistral", NewMistralEmbedder("k", "", 0), "mistral-embed", 1024},
		{"jina", NewJinaEmbedder("k", "", 0), "jina-embeddings-v3", 1024},
		{"google", NewGoogleEmbedder("k", "", 0), "text-embedding-004", 768},
		{"cohere", NewCohereEmbedder("k", "", 0), "embed-english-v3.0", 1024},
		{"voyageai", NewVoyageEmbedder("k", "", 0), "voyage-3", 1024},
	}
	for _, c := range cases {
		if c.e.GetID() != c.id {
			t.Errorf("%s: ID = %q, want %q", c.name, c.e.GetID(), c.id)
		}
		if c.e.GetDimensions() != c.dims {
			t.Errorf("%s: dims = %d, want %d", c.name, c.e.GetDimensions(), c.dims)
		}
	}
}

func TestEmbedderEmptyTextErrors(t *testing.T) {
	// Native embedders should reject empty text before any network call.
	if _, err := NewGoogleEmbedder("k", "", 0).GetEmbedding(""); err != ErrEmptyText {
		t.Errorf("google: expected ErrEmptyText, got %v", err)
	}
	if _, err := NewCohereEmbedder("k", "", 0).GetEmbedding(""); err != ErrEmptyText {
		t.Errorf("cohere: expected ErrEmptyText, got %v", err)
	}
	if _, err := NewVoyageEmbedder("k", "", 0).GetEmbedding(""); err != ErrEmptyText {
		t.Errorf("voyage: expected ErrEmptyText, got %v", err)
	}
}

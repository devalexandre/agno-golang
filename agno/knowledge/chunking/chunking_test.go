package chunking

import (
	"strings"
	"testing"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
)

func doc(content string) *document.Document {
	d := document.NewDocument(content)
	d.ID = "parent"
	d.Name = "Doc"
	return d
}

// All strategies must satisfy the interface and produce chunks with correct
// ChunkIndex/ChunkTotal/ParentID bookkeeping.
func TestAllStrategiesImplementInterface(t *testing.T) {
	strategies := map[string]document.ChunkingStrategy{
		"fixed":     NewFixedChunking(50, 10),
		"recursive": NewRecursiveChunking(50, 10),
		"document":  NewDocumentChunking(50, 10),
		"markdown":  NewMarkdownChunking(50, 10),
		"code":      NewCodeChunking(50, 10),
		"row":       NewRowChunking(2, false),
		"semantic":  NewSemanticChunking(embedder.NewMockEmbedder(16), 50, 0.5),
		"agentic":   NewAgenticChunking(nil, 50, 10), // nil model -> recursive fallback
	}

	text := strings.Repeat("This is a sentence. ", 20)
	for name, s := range strategies {
		chunks, err := s.Chunk(doc(text))
		if err != nil {
			t.Errorf("%s: Chunk error: %v", name, err)
			continue
		}
		if len(chunks) == 0 {
			t.Errorf("%s: produced no chunks", name)
			continue
		}
		for i, ch := range chunks {
			if ch.ChunkIndex != i {
				t.Errorf("%s: chunk %d has ChunkIndex %d", name, i, ch.ChunkIndex)
			}
			if ch.ChunkTotal != len(chunks) {
				t.Errorf("%s: chunk %d has ChunkTotal %d, want %d", name, i, ch.ChunkTotal, len(chunks))
			}
			if ch.ParentID != "parent" {
				t.Errorf("%s: chunk %d ParentID = %q", name, i, ch.ParentID)
			}
		}
	}
}

func TestFixedChunkingSizes(t *testing.T) {
	d := doc(strings.Repeat("a", 250))
	chunks, _ := NewFixedChunking(100, 20).Chunk(d)
	// step = 80; windows [0:100], [80:180], [160:250] -> 3 chunks cover 250 runes
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	for _, ch := range chunks {
		if len([]rune(ch.Content)) > 100 {
			t.Fatalf("chunk exceeds size: %d", len([]rune(ch.Content)))
		}
	}
}

func TestRowChunkingHeader(t *testing.T) {
	csv := "name,age\nalice,30\nbob,25\ncarol,40"
	chunks, _ := NewRowChunking(1, true).Chunk(doc(csv))
	if len(chunks) != 3 {
		t.Fatalf("expected 3 data-row chunks, got %d", len(chunks))
	}
	for _, ch := range chunks {
		if !strings.HasPrefix(ch.Content, "name,age\n") {
			t.Fatalf("row chunk missing header: %q", ch.Content)
		}
	}
}

func TestMarkdownChunkingSections(t *testing.T) {
	md := "# Title\nintro\n\n## Section A\ncontent a\n\n## Section B\ncontent b"
	chunks, _ := NewMarkdownChunking(40, 0).Chunk(doc(md))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple sections, got %d", len(chunks))
	}
}

func TestDocumentChunkingParagraphs(t *testing.T) {
	text := "Para one is here.\n\nPara two is here.\n\nPara three is here."
	chunks, _ := NewDocumentChunking(30, 0).Chunk(doc(text))
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
}

func TestCosineSimilarity(t *testing.T) {
	if got := cosineSimilarity([]float64{1, 0}, []float64{1, 0}); got < 0.999 {
		t.Fatalf("identical vectors cosine = %f, want ~1", got)
	}
	if got := cosineSimilarity([]float64{1, 0}, []float64{0, 1}); got > 0.001 {
		t.Fatalf("orthogonal vectors cosine = %f, want ~0", got)
	}
}

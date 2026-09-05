package chunking

import (
	"math"
	"regexp"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
)

// SemanticChunking groups consecutive sentences while their embedding similarity
// stays above SimilarityThreshold, starting a new chunk on a semantic shift or
// when the running chunk would exceed ChunkSize.
type SemanticChunking struct {
	Embedder            embedder.Embedder
	ChunkSize           int
	SimilarityThreshold float64
}

var sentenceSplit = regexp.MustCompile(`(?s)(.*?[.!?]+(?:\s+|$))`)

// NewSemanticChunking creates a semantic chunker. threshold is the minimum
// cosine similarity (0..1) to keep two adjacent sentences in the same chunk.
func NewSemanticChunking(emb embedder.Embedder, chunkSize int, threshold float64) *SemanticChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if threshold <= 0 {
		threshold = 0.5
	}
	return &SemanticChunking{Embedder: emb, ChunkSize: chunkSize, SimilarityThreshold: threshold}
}

func (c *SemanticChunking) GetChunkSize() int { return c.ChunkSize }
func (c *SemanticChunking) GetOverlap() int   { return 0 }

// Chunk implements document.ChunkingStrategy.
func (c *SemanticChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	sentences := splitSentences(doc.Content)
	if len(sentences) == 0 {
		return nil, nil
	}
	if c.Embedder == nil {
		// Without an embedder we cannot measure similarity; fall back to packing.
		merged := mergePieces(sentences, c.ChunkSize, 0, " ")
		return buildChunks(doc, merged), nil
	}

	var pieces []string
	var cur []string
	var prevEmb []float64

	flush := func() {
		if len(cur) > 0 {
			pieces = append(pieces, strings.Join(cur, " "))
			cur = nil
		}
	}

	for _, s := range sentences {
		emb, err := c.Embedder.GetEmbedding(s)
		if err != nil {
			return nil, err
		}
		curLen := len([]rune(strings.Join(cur, " ")))
		if len(cur) > 0 {
			sim := cosineSimilarity(prevEmb, emb)
			if sim < c.SimilarityThreshold || curLen+len([]rune(s)) > c.ChunkSize {
				flush()
			}
		}
		cur = append(cur, s)
		prevEmb = emb
	}
	flush()

	return buildChunks(doc, pieces), nil
}

func splitSentences(text string) []string {
	matches := sentenceSplit.FindAllString(text, -1)
	var out []string
	for _, m := range matches {
		if t := strings.TrimSpace(m); t != "" {
			out = append(out, t)
		}
	}
	// Capture trailing text with no terminal punctuation.
	joined := strings.Join(out, " ")
	if rest := strings.TrimSpace(strings.TrimPrefix(text, joined)); rest != "" && len(out) == 0 {
		out = append(out, strings.TrimSpace(text))
	}
	return out
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

var _ document.ChunkingStrategy = (*SemanticChunking)(nil)

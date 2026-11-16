package chunking

import (
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// RecursiveChunking splits text on a hierarchy of separators (paragraphs, lines,
// sentences, words), keeping pieces under ChunkSize, then packs them with overlap.
type RecursiveChunking struct {
	ChunkSize  int
	Overlap    int
	Separators []string
}

// NewRecursiveChunking creates a recursive chunker with sensible default separators.
func NewRecursiveChunking(chunkSize, overlap int) *RecursiveChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	return &RecursiveChunking{
		ChunkSize:  chunkSize,
		Overlap:    overlap,
		Separators: []string{"\n\n", "\n", ". ", " "},
	}
}

func (c *RecursiveChunking) GetChunkSize() int { return c.ChunkSize }
func (c *RecursiveChunking) GetOverlap() int   { return c.Overlap }

// Chunk implements document.ChunkingStrategy.
func (c *RecursiveChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	pieces := recursiveSplit(doc.Content, c.Separators, c.ChunkSize)
	merged := mergePieces(pieces, c.ChunkSize, c.Overlap, " ")
	return buildChunks(doc, merged), nil
}

func recursiveSplit(text string, seps []string, chunkSize int) []string {
	if len([]rune(text)) <= chunkSize {
		return []string{text}
	}
	if len(seps) == 0 {
		return splitFixed([]rune(text), chunkSize, 0)
	}
	var out []string
	for _, part := range strings.Split(text, seps[0]) {
		if strings.TrimSpace(part) == "" {
			continue
		}
		if len([]rune(part)) <= chunkSize {
			out = append(out, part)
		} else {
			out = append(out, recursiveSplit(part, seps[1:], chunkSize)...)
		}
	}
	return out
}

var _ document.ChunkingStrategy = (*RecursiveChunking)(nil)

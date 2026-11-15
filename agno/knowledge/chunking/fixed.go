package chunking

import "github.com/devalexandre/agno-golang/agno/document"

// FixedChunking splits content into fixed-size character windows with overlap.
type FixedChunking struct {
	ChunkSize int
	Overlap   int
}

// NewFixedChunking creates a fixed-size chunker.
func NewFixedChunking(chunkSize, overlap int) *FixedChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	return &FixedChunking{ChunkSize: chunkSize, Overlap: overlap}
}

func (c *FixedChunking) GetChunkSize() int { return c.ChunkSize }
func (c *FixedChunking) GetOverlap() int   { return c.Overlap }

// Chunk implements document.ChunkingStrategy.
func (c *FixedChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	pieces := splitFixed([]rune(doc.Content), c.ChunkSize, c.Overlap)
	return buildChunks(doc, pieces), nil
}

var _ document.ChunkingStrategy = (*FixedChunking)(nil)

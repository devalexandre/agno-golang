package chunking

import (
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// DocumentChunking splits on paragraph boundaries (blank lines), packing
// paragraphs into chunks up to ChunkSize while preserving paragraph structure.
type DocumentChunking struct {
	ChunkSize int
	Overlap   int
}

// NewDocumentChunking creates a paragraph-aware chunker.
func NewDocumentChunking(chunkSize, overlap int) *DocumentChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	return &DocumentChunking{ChunkSize: chunkSize, Overlap: overlap}
}

func (c *DocumentChunking) GetChunkSize() int { return c.ChunkSize }
func (c *DocumentChunking) GetOverlap() int   { return c.Overlap }

// Chunk implements document.ChunkingStrategy.
func (c *DocumentChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	paragraphs := strings.Split(doc.Content, "\n\n")
	merged := mergePieces(paragraphs, c.ChunkSize, c.Overlap, "\n\n")
	return buildChunks(doc, merged), nil
}

var _ document.ChunkingStrategy = (*DocumentChunking)(nil)

package chunking

import (
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// MarkdownChunking splits content on Markdown headings, keeping each heading with
// its section, then packs sections into chunks up to ChunkSize.
type MarkdownChunking struct {
	ChunkSize int
	Overlap   int
}

// NewMarkdownChunking creates a Markdown-aware chunker.
func NewMarkdownChunking(chunkSize, overlap int) *MarkdownChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	return &MarkdownChunking{ChunkSize: chunkSize, Overlap: overlap}
}

func (c *MarkdownChunking) GetChunkSize() int { return c.ChunkSize }
func (c *MarkdownChunking) GetOverlap() int   { return c.Overlap }

// Chunk implements document.ChunkingStrategy.
func (c *MarkdownChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	sections := splitMarkdownSections(doc.Content)
	merged := mergePieces(sections, c.ChunkSize, c.Overlap, "\n\n")
	return buildChunks(doc, merged), nil
}

func splitMarkdownSections(text string) []string {
	lines := strings.Split(text, "\n")
	var sections []string
	var cur strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && cur.Len() > 0 {
			sections = append(sections, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	if cur.Len() > 0 {
		sections = append(sections, strings.TrimRight(cur.String(), "\n"))
	}
	return sections
}

var _ document.ChunkingStrategy = (*MarkdownChunking)(nil)

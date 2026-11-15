package chunking

import (
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// codeBoundaryPrefixes start a new logical code block when found at the start of
// a (trimmed) line. Covers common definitions across popular languages.
var codeBoundaryPrefixes = []string{
	"func ", "function ", "def ", "class ", "type ", "struct ", "interface ",
	"public ", "private ", "protected ", "static ", "async ", "export ", "package ",
}

// CodeChunking splits source code on blank lines and top-level definition
// boundaries, packing blocks into chunks up to ChunkSize.
type CodeChunking struct {
	ChunkSize int
	Overlap   int
}

// NewCodeChunking creates a code-aware chunker.
func NewCodeChunking(chunkSize, overlap int) *CodeChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	return &CodeChunking{ChunkSize: chunkSize, Overlap: overlap}
}

func (c *CodeChunking) GetChunkSize() int { return c.ChunkSize }
func (c *CodeChunking) GetOverlap() int   { return c.Overlap }

// Chunk implements document.ChunkingStrategy.
func (c *CodeChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	blocks := splitCodeBlocks(doc.Content)
	merged := mergePieces(blocks, c.ChunkSize, c.Overlap, "\n\n")
	return buildChunks(doc, merged), nil
}

func splitCodeBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var blocks []string
	var cur strings.Builder
	blankRun := false

	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			blocks = append(blocks, strings.TrimRight(cur.String(), "\n"))
		}
		cur.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			blankRun = true
			cur.WriteString(line)
			cur.WriteString("\n")
			continue
		}

		isBoundary := false
		for _, p := range codeBoundaryPrefixes {
			if strings.HasPrefix(trimmed, p) {
				isBoundary = true
				break
			}
		}
		// Start a new block at a definition that follows blank space, or after a blank run.
		if (isBoundary || blankRun) && cur.Len() > 0 && strings.TrimSpace(cur.String()) != "" {
			if isBoundary {
				flush()
			}
		}
		blankRun = false
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	flush()
	return blocks
}

var _ document.ChunkingStrategy = (*CodeChunking)(nil)

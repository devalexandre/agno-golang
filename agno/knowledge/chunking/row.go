package chunking

import (
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// RowChunking splits row-oriented content (CSV/TSV/tables) by lines, grouping
// RowsPerChunk data rows per chunk and optionally repeating the header row.
type RowChunking struct {
	RowsPerChunk  int
	IncludeHeader bool
}

// NewRowChunking creates a row-based chunker. rowsPerChunk defaults to 1.
func NewRowChunking(rowsPerChunk int, includeHeader bool) *RowChunking {
	if rowsPerChunk <= 0 {
		rowsPerChunk = 1
	}
	return &RowChunking{RowsPerChunk: rowsPerChunk, IncludeHeader: includeHeader}
}

// GetChunkSize reports the number of rows per chunk.
func (c *RowChunking) GetChunkSize() int { return c.RowsPerChunk }

// GetOverlap is always 0 for row chunking.
func (c *RowChunking) GetOverlap() int { return 0 }

// Chunk implements document.ChunkingStrategy.
func (c *RowChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	var lines []string
	for _, l := range strings.Split(doc.Content, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return nil, nil
	}

	header := ""
	start := 0
	if c.IncludeHeader {
		header = lines[0]
		start = 1
	}

	var pieces []string
	for i := start; i < len(lines); i += c.RowsPerChunk {
		end := i + c.RowsPerChunk
		if end > len(lines) {
			end = len(lines)
		}
		group := strings.Join(lines[i:end], "\n")
		if header != "" {
			group = header + "\n" + group
		}
		pieces = append(pieces, group)
	}
	return buildChunks(doc, pieces), nil
}

var _ document.ChunkingStrategy = (*RowChunking)(nil)

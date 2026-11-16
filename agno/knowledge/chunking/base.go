// Package chunking provides document chunking strategies implementing
// document.ChunkingStrategy. Strategies split a Document's content into smaller
// chunk Documents (carrying ChunkIndex/ChunkTotal/ParentID) ready for embedding
// and indexing. Mirrors agno-python/knowledge/chunking.
package chunking

import (
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

const (
	// DefaultChunkSize is the default maximum chunk size in characters.
	DefaultChunkSize = 1000
	// DefaultOverlap is the default overlap between consecutive chunks.
	DefaultOverlap = 100
)

// buildChunks turns text pieces into chunk Documents derived from parent.
// Empty/whitespace-only pieces are skipped.
func buildChunks(parent *document.Document, pieces []string) []*document.Document {
	cleaned := make([]string, 0, len(pieces))
	for _, p := range pieces {
		if strings.TrimSpace(p) != "" {
			cleaned = append(cleaned, p)
		}
	}

	total := len(cleaned)
	docs := make([]*document.Document, 0, total)
	for i, p := range cleaned {
		c := parent.Clone()
		c.Content = p
		c.ChunkIndex = i
		c.ChunkTotal = total
		c.ParentID = parent.ID
		if parent.ID != "" {
			c.ID = fmt.Sprintf("%s_chunk_%d", parent.ID, i)
		}
		if parent.Name != "" {
			c.Name = fmt.Sprintf("%s (%d/%d)", parent.Name, i+1, total)
		}
		docs = append(docs, c)
	}
	return docs
}

// mergePieces packs adjacent pieces into chunks no larger than chunkSize
// (measured in runes), joining them with sep and carrying `overlap` runes of
// trailing context from the previous chunk into the next. Pieces larger than
// chunkSize are hard-split via splitFixed.
func mergePieces(pieces []string, chunkSize, overlap int, sep string) []string {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	sepLen := len([]rune(sep))

	var out []string
	var cur strings.Builder
	curLen := 0

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
			curLen = 0
		}
	}

	for _, piece := range pieces {
		pieceRunes := len([]rune(piece))

		// Hard-split oversized pieces.
		if pieceRunes > chunkSize {
			flush()
			out = append(out, splitFixed([]rune(piece), chunkSize, overlap)...)
			continue
		}

		addLen := pieceRunes
		if cur.Len() > 0 {
			addLen += sepLen
		}
		if curLen+addLen > chunkSize {
			// Carry overlap from the end of the current chunk into the next.
			prev := cur.String()
			flush()
			if ov := tailRunes(prev, overlap); ov != "" {
				cur.WriteString(ov)
				curLen = len([]rune(ov))
			}
		}
		if cur.Len() > 0 {
			cur.WriteString(sep)
			curLen += sepLen
		}
		cur.WriteString(piece)
		curLen += pieceRunes
	}
	flush()
	return out
}

// splitFixed splits a rune slice into windows of size `size` with `overlap`.
func splitFixed(runes []rune, size, overlap int) []string {
	if size <= 0 {
		size = DefaultChunkSize
	}
	if overlap < 0 || overlap >= size {
		overlap = 0
	}
	step := size - overlap
	var out []string
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[start:end]))
		if end == len(runes) {
			break
		}
	}
	return out
}

// tailRunes returns the last n runes of s.
func tailRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

func runeLen(s string) int { return len([]rune(s)) }

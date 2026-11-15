package chunking

import (
	"context"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/models"
)

// chunkDelimiter is the marker the model is asked to insert between chunks.
const chunkDelimiter = "<<<CHUNK>>>"

// AgenticChunking asks an LLM to decide semantic chunk boundaries. It is more
// expensive than the deterministic strategies but adapts to content structure.
// If the model is nil or fails, it falls back to RecursiveChunking.
type AgenticChunking struct {
	Model     models.AgnoModelInterface
	ChunkSize int
	Overlap   int
}

// NewAgenticChunking creates an agentic chunker backed by model.
func NewAgenticChunking(model models.AgnoModelInterface, chunkSize, overlap int) *AgenticChunking {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	return &AgenticChunking{Model: model, ChunkSize: chunkSize, Overlap: overlap}
}

func (c *AgenticChunking) GetChunkSize() int { return c.ChunkSize }
func (c *AgenticChunking) GetOverlap() int   { return c.Overlap }

// Chunk implements document.ChunkingStrategy.
func (c *AgenticChunking) Chunk(doc *document.Document) ([]*document.Document, error) {
	if c.Model == nil {
		return c.fallback(doc)
	}

	system := "You split documents into semantically coherent chunks. " +
		"Return the original text unchanged, inserting the exact marker " +
		chunkDelimiter + " between chunks. Each chunk should cover one coherent " +
		"idea and stay under roughly " + strconv.Itoa(c.ChunkSize) + " characters. " +
		"Do not add commentary."

	messages := []models.Message{
		{Role: models.TypeSystemRole, Content: system},
		{Role: models.TypeUserRole, Content: doc.Content},
	}

	resp, err := c.Model.Invoke(context.Background(), messages)
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return c.fallback(doc)
	}

	parts := strings.Split(resp.Content, chunkDelimiter)
	if len(parts) <= 1 {
		// Model didn't insert markers — fall back to a deterministic split.
		return c.fallback(doc)
	}

	pieces := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			pieces = append(pieces, strings.TrimSpace(p))
		}
	}
	return buildChunks(doc, pieces), nil
}

func (c *AgenticChunking) fallback(doc *document.Document) ([]*document.Document, error) {
	return NewRecursiveChunking(c.ChunkSize, c.Overlap).Chunk(doc)
}

var _ document.ChunkingStrategy = (*AgenticChunking)(nil)

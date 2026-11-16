package chunking

import (
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/models"
)

// StrategyType names a chunking strategy. Values match agno-python's
// ChunkingStrategyType so configs can be shared.
type StrategyType string

const (
	AgenticChunker   StrategyType = "AgenticChunker"
	CodeChunker      StrategyType = "CodeChunker"
	DocumentChunker  StrategyType = "DocumentChunker"
	RecursiveChunker StrategyType = "RecursiveChunker"
	SemanticChunker  StrategyType = "SemanticChunker"
	FixedSizeChunker StrategyType = "FixedSizeChunker"
	RowChunker       StrategyType = "RowChunker"
	MarkdownChunker  StrategyType = "MarkdownChunker"
	// PageChunker is PageMarkdownChunking (Python exposes it as a class only).
	PageChunker StrategyType = "PageChunker"
)

// AllStrategyTypes lists every StrategyType.
func AllStrategyTypes() []StrategyType {
	return []StrategyType{AgenticChunker, CodeChunker, DocumentChunker, RecursiveChunker, SemanticChunker,
		FixedSizeChunker, RowChunker, MarkdownChunker, PageChunker}
}

// ParseStrategyType converts a name to a StrategyType: the exact value, a
// case-insensitive match, or a short alias ("fixed", "fixed_size",
// "markdown", "page", ...).
func ParseStrategyType(name string) (StrategyType, error) {
	clean := strings.TrimSpace(name)
	for _, t := range AllStrategyTypes() {
		if string(t) == clean {
			return t, nil
		}
	}
	norm := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(clean))
	norm = strings.TrimSuffix(norm, "chunker")
	norm = strings.TrimSuffix(norm, "chunking")
	aliases := map[string]StrategyType{
		"agentic": AgenticChunker, "code": CodeChunker, "document": DocumentChunker,
		"recursive": RecursiveChunker, "semantic": SemanticChunker, "fixed": FixedSizeChunker,
		"fixedsize": FixedSizeChunker, "row": RowChunker, "markdown": MarkdownChunker,
		"page": PageChunker, "pagemarkdown": PageChunker,
	}
	if t, ok := aliases[norm]; ok {
		return t, nil
	}
	return "", fmt.Errorf("chunking: unsupported strategy %q", name)
}

// StrategyOptions configures NewStrategy. Zero values mean defaults.
type StrategyOptions struct {
	ChunkSize int
	Overlap   int
	// Model is required by AgenticChunker.
	Model models.AgnoModelInterface
	// Embedder is required by SemanticChunker.
	Embedder embedder.Embedder
	// SimilarityThreshold for SemanticChunker.
	SimilarityThreshold float64
	// RowsPerChunk / IncludeHeader for RowChunker.
	RowsPerChunk  int
	IncludeHeader bool
	// SplitLevel for PageChunker (default 2).
	SplitLevel int
}

// NewStrategy builds a chunking strategy by type, like Python's
// ChunkingStrategyFactory.create_strategy.
func NewStrategy(t StrategyType, o StrategyOptions) (document.ChunkingStrategy, error) {
	switch t {
	case AgenticChunker:
		if o.Model == nil {
			return nil, fmt.Errorf("chunking: %s requires a Model", t)
		}
		return NewAgenticChunking(o.Model, o.ChunkSize, o.Overlap), nil
	case CodeChunker:
		return NewCodeChunking(o.ChunkSize, o.Overlap), nil
	case DocumentChunker:
		return NewDocumentChunking(o.ChunkSize, o.Overlap), nil
	case RecursiveChunker:
		return NewRecursiveChunking(o.ChunkSize, o.Overlap), nil
	case SemanticChunker:
		if o.Embedder == nil {
			return nil, fmt.Errorf("chunking: %s requires an Embedder", t)
		}
		return NewSemanticChunking(o.Embedder, o.ChunkSize, o.SimilarityThreshold), nil
	case FixedSizeChunker:
		return NewFixedChunking(o.ChunkSize, o.Overlap), nil
	case RowChunker:
		return NewRowChunking(o.RowsPerChunk, o.IncludeHeader), nil
	case MarkdownChunker:
		return NewMarkdownChunking(o.ChunkSize, o.Overlap), nil
	case PageChunker:
		return NewPageMarkdownChunking(o.ChunkSize, o.SplitLevel), nil
	default:
		return nil, fmt.Errorf("chunking: unsupported strategy %q", t)
	}
}

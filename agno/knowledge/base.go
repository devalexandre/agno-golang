package knowledge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// Knowledge is the base interface for knowledge bases
type Knowledge interface {
	// Load loads documents into the knowledge base
	Load(ctx context.Context, recreate bool) error

	// LoadDocument loads a specific document
	LoadDocument(ctx context.Context, doc document.Document) error

	// Search searches for documents in the knowledge base
	Search(ctx context.Context, query string, numDocuments int) ([]*SearchResult, error)

	// Drop removes all documents from the base
	Drop(ctx context.Context) error

	// Exists checks if the knowledge base exists
	Exists(ctx context.Context) (bool, error)

	// GetCount returns the number of documents in the base
	GetCount(ctx context.Context) (int64, error)

	// GetInfo returns information about the knowledge base
	GetInfo() KnowledgeInfo
}

// VectorDB is an alias for vectordb.VectorDB for native compatibility like in Python Agno
type VectorDB = vectordb.VectorDB

// SearchResult is an alias for vectordb.SearchResult for native compatibility
type SearchResult = vectordb.SearchResult

// KnowledgeInfo contains information about the knowledge base
type KnowledgeInfo struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SearchFilters define filters for search
type SearchFilters struct {
	Include map[string]interface{} `json:"include,omitempty"`
	Exclude map[string]interface{} `json:"exclude,omitempty"`
}

// BaseKnowledge base implementation for knowledge bases
type BaseKnowledge struct {
	Name         string
	VectorDB     VectorDB
	Embedder     embedder.Embedder
	NumDocuments int
	Filters      *SearchFilters
	Recreate     bool
	Metadata     map[string]interface{}
}

// NewBaseKnowledge creates a new BaseKnowledge instance
func NewBaseKnowledge(name string, vectorDB VectorDB) *BaseKnowledge {
	return &BaseKnowledge{
		Name:         name,
		VectorDB:     vectorDB,
		NumDocuments: 5,
		Metadata:     make(map[string]interface{}),
	}
}

// GetInfo returns information about the knowledge base
func (k *BaseKnowledge) GetInfo() KnowledgeInfo {
	return KnowledgeInfo{
		Name:        k.Name,
		Type:        "base",
		Description: fmt.Sprintf("Base knowledge: %s", k.Name),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    k.Metadata,
	}
}

// SearchDocuments searches documents with filters
func (k *BaseKnowledge) SearchDocuments(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]document.Document, error) {
	if k.VectorDB == nil {
		return nil, fmt.Errorf("vector database not configured")
	}

	results, err := k.VectorDB.Search(ctx, query, numDocuments, filters)
	if err != nil {
		return nil, err
	}

	// Convert SearchResult to Document slice
	docs := make([]document.Document, len(results))
	for i, result := range results {
		docs[i] = *result.Document
	}

	return docs, nil
}

// LoadDocuments loads documents into the knowledge base
func (k *BaseKnowledge) LoadDocuments(ctx context.Context, docs []document.Document, recreate bool) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Check if should recreate
	if recreate {
		if err := k.VectorDB.Drop(ctx); err != nil {
			// Ignore error if doesn't exist
		}
	}

	// Create table if doesn't exist
	if err := k.VectorDB.Create(ctx); err != nil {
		fmt.Println("There was VecctorDb skip create")
	}

	// Insert documents
	if len(docs) > 0 {
		// Convert []document.Document to []*document.Document
		docPtrs := make([]*document.Document, len(docs))
		for i := range docs {
			docPtrs[i] = &docs[i]
		}
		return k.VectorDB.Insert(ctx, docPtrs, nil)
	}

	return nil
}

// Search implementa Knowledge interface
func (k *BaseKnowledge) Search(ctx context.Context, query string, numDocuments int) ([]document.Document, error) {
	if numDocuments <= 0 {
		numDocuments = k.NumDocuments
	}

	var filters map[string]interface{}
	if k.Filters != nil {
		filters = k.Filters.Include
	}

	return k.SearchDocuments(ctx, query, numDocuments, filters)
}

// Add adds documents to the knowledge base
func (k *BaseKnowledge) Add(ctx context.Context, documents []document.Document) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	// Convert []document.Document to []*document.Document
	docPtrs := make([]*document.Document, len(documents))
	for i := range documents {
		docPtrs[i] = &documents[i]
	}

	return k.VectorDB.Insert(ctx, docPtrs, nil)
}

// Exists checks if the knowledge base exists
func (k *BaseKnowledge) Exists(ctx context.Context) (bool, error) {
	if k.VectorDB == nil {
		return false, fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Exists(ctx)
}

// Drop removes the knowledge base
func (k *BaseKnowledge) Drop(ctx context.Context) error {
	if k.VectorDB == nil {
		return fmt.Errorf("vector database not configured")
	}

	return k.VectorDB.Drop(ctx)
}

// Load implements Knowledge interface
func (k *BaseKnowledge) Load(ctx context.Context, recreate bool) error {
	// Empty default implementation - should be overridden by subclasses
	return nil
}

// SetEmbedder configures the embedder
func (k *BaseKnowledge) SetEmbedder(emb embedder.Embedder) {
	k.Embedder = emb
	// Note: VectorDB implementation should handle embedder internally
}

// GetEmbedder returns the configured embedder
func (k *BaseKnowledge) GetEmbedder() embedder.Embedder {
	if k.Embedder != nil {
		return k.Embedder
	}
	if k.VectorDB != nil {
		return k.VectorDB.GetEmbedder()
	}
	return nil
}

// ValidateDocuments validates documents before processing
func ValidateDocuments(docs []document.Document) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to process")
	}

	for i, doc := range docs {
		if doc.ID == "" {
			return fmt.Errorf("document at index %d has empty ID", i)
		}
		if doc.Content == "" {
			return fmt.Errorf("document at index %d has empty content", i)
		}
	}

	return nil
}

// SanitizeFileName sanitizes filename to use as collection/table name
func SanitizeFileName(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))

	// Replace special characters with underscore
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")

	// Ensure it starts with a letter
	if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		name = "kb_" + name
	}

	// Convert to lowercase
	return strings.ToLower(name)
}

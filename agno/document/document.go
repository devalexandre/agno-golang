package document

import (
	"encoding/json"
	"time"
)

// Document represents a document in the knowledge base
type Document struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata"`
	Source      string                 `json:"source"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`

	// Vector embeddings (if available)
	Embeddings []float64 `json:"embeddings,omitempty"`

	// Chunking information
	ChunkIndex int    `json:"chunk_index,omitempty"`
	ChunkTotal int    `json:"chunk_total,omitempty"`
	ParentID   string `json:"parent_id,omitempty"`
}

// NewDocument creates a new document
func NewDocument(content string) *Document {
	return &Document{
		Content:     content,
		ContentType: "text/plain",
		Metadata:    make(map[string]interface{}),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// NewDocumentWithMetadata creates a new document with metadata
func NewDocumentWithMetadata(content string, metadata map[string]interface{}) *Document {
	doc := NewDocument(content)
	doc.Metadata = metadata
	return doc
}

// ToJSON converts the document to JSON
func (d *Document) ToJSON() (string, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON creates a document from JSON
func FromJSON(jsonData string) (*Document, error) {
	var doc Document
	err := json.Unmarshal([]byte(jsonData), &doc)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// Clone creates a copy of the document
func (d *Document) Clone() *Document {
	// Deep copy metadata
	metadata := make(map[string]interface{})
	for k, v := range d.Metadata {
		metadata[k] = v
	}

	// Deep copy embeddings
	embeddings := make([]float64, len(d.Embeddings))
	copy(embeddings, d.Embeddings)

	return &Document{
		ID:          d.ID,
		Name:        d.Name,
		Content:     d.Content,
		ContentType: d.ContentType,
		Metadata:    metadata,
		Source:      d.Source,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   time.Now(),
		Embeddings:  embeddings,
		ChunkIndex:  d.ChunkIndex,
		ChunkTotal:  d.ChunkTotal,
		ParentID:    d.ParentID,
	}
}

// AddMetadata adds metadata to the document
func (d *Document) AddMetadata(key string, value interface{}) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]interface{})
	}
	d.Metadata[key] = value
	d.UpdatedAt = time.Now()
}

// GetMetadata gets metadata from the document
func (d *Document) GetMetadata(key string) (interface{}, bool) {
	if d.Metadata == nil {
		return nil, false
	}
	value, exists := d.Metadata[key]
	return value, exists
}

// SetEmbeddings sets the embeddings for the document
func (d *Document) SetEmbeddings(embeddings []float64) {
	d.Embeddings = embeddings
	d.UpdatedAt = time.Now()
}

// HasEmbeddings checks if the document has embeddings
func (d *Document) HasEmbeddings() bool {
	return len(d.Embeddings) > 0
}

// IsChunk checks if the document is a chunk of a larger document
func (d *Document) IsChunk() bool {
	return d.ParentID != ""
}

// GetLength returns the length of the document content
func (d *Document) GetLength() int {
	return len(d.Content)
}

// DocumentReader defines interface for reading documents from various sources
type DocumentReader interface {
	// Read reads documents from a source
	Read(source string) ([]*Document, error)

	// ReadAsync reads documents asynchronously
	ReadAsync(source string) (<-chan *Document, <-chan error)

	// SupportedFormats returns the file formats supported by this reader
	SupportedFormats() []string
}

// DocumentProcessor defines interface for processing documents
type DocumentProcessor interface {
	// Process processes a document (e.g., chunking, embedding)
	Process(doc *Document) ([]*Document, error)

	// ProcessBatch processes multiple documents
	ProcessBatch(docs []*Document) ([]*Document, error)
}

// ChunkingStrategy defines interface for document chunking strategies
type ChunkingStrategy interface {
	// Chunk splits a document into smaller chunks
	Chunk(doc *Document) ([]*Document, error)

	// GetChunkSize returns the maximum chunk size
	GetChunkSize() int

	// GetOverlap returns the overlap between chunks
	GetOverlap() int
}

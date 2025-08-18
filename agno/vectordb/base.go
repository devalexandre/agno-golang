package vectordb

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
)

// Distance represents the distance metric used for vector similarity
type Distance string

const (
	DistanceL2              Distance = "l2"
	DistanceCosine          Distance = "cosine"
	DistanceMaxInnerProduct Distance = "max_inner_product"
	DistanceEuclidean       Distance = "euclidean"
	DistanceDot             Distance = "dot"
)

// SearchType represents the type of search to perform
type SearchType string

const (
	SearchTypeVector  SearchType = "vector"
	SearchTypeKeyword SearchType = "keyword"
	SearchTypeHybrid  SearchType = "hybrid"
)

// SearchResult represents a search result with score
type SearchResult struct {
	Document *document.Document `json:"document"`
	Score    float64            `json:"score"`
	Distance float64            `json:"distance"`
}

// VectorDB defines the interface for vector database operations
type VectorDB interface {
	// Collection Management
	Create(ctx context.Context) error
	Exists(ctx context.Context) (bool, error)
	Drop(ctx context.Context) error
	Optimize(ctx context.Context) error

	// Document Operations
	Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error
	Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error
	Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)
	VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)
	KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)
	HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*SearchResult, error)

	// Utility Methods
	GetCount(ctx context.Context) (int64, error)
	GetEmbedder() embedder.Embedder

	// Document Existence Checks
	DocExists(ctx context.Context, doc *document.Document) (bool, error)
	NameExists(ctx context.Context, name string) (bool, error)
	IDExists(ctx context.Context, id string) (bool, error)
}

// BaseVectorDB provides common functionality for VectorDB implementations
type BaseVectorDB struct {
	Embedder   embedder.Embedder `json:"embedder"`
	SearchType SearchType        `json:"search_type"`
	Distance   Distance          `json:"distance"`
	Dimensions int               `json:"dimensions"`
}

// NewBaseVectorDB creates a new BaseVectorDB instance
func NewBaseVectorDB(emb embedder.Embedder, searchType SearchType, distance Distance) *BaseVectorDB {
	dimensions := 0
	if emb != nil {
		dimensions = emb.GetDimensions()
	}

	return &BaseVectorDB{
		Embedder:   emb,
		SearchType: searchType,
		Distance:   distance,
		Dimensions: dimensions,
	}
}

// GetEmbedder returns the embedder instance
func (b *BaseVectorDB) GetEmbedder() embedder.Embedder {
	return b.Embedder
}

// EmbedDocuments generates embeddings for a list of documents
func (b *BaseVectorDB) EmbedDocuments(docs []*document.Document) error {
	if b.Embedder == nil {
		return nil // No embedder configured
	}

	for _, doc := range docs {
		if doc.Embeddings == nil || len(doc.Embeddings) == 0 {
			embedding, err := b.Embedder.GetEmbedding(doc.Content)
			if err != nil {
				return err
			}
			doc.Embeddings = embedding
		}
	}

	return nil
}

// EmbedQuery generates embedding for a search query
func (b *BaseVectorDB) EmbedQuery(query string) ([]float64, error) {
	if b.Embedder == nil {
		return nil, nil
	}

	return b.Embedder.GetEmbedding(query)
}

// CalculateCosineSimilarity calculates cosine similarity between two vectors
func CalculateCosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}

// CalculateEuclideanDistance calculates Euclidean distance between two vectors
func CalculateEuclideanDistance(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var sum float64
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}

	return sum // Return squared distance for efficiency
}

// CalculateDotProduct calculates dot product between two vectors
func CalculateDotProduct(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct float64
	for i := range a {
		dotProduct += a[i] * b[i]
	}

	return dotProduct
}

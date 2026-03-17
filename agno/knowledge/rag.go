package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
)

// Reranker is the interface for document reranking models.
type Reranker interface {
	Rerank(ctx context.Context, query string, results []*SearchResult) ([]*SearchResult, error)
}

// RAGPipeline implements a Retrieval-Augmented Generation pipeline
type RAGPipeline struct {
	KnowledgeBase    Knowledge
	NumDocuments     int
	MaxContextLength int
	Reranker         Reranker
}

// RAGResult represents the result of a RAG operation
type RAGResult struct {
	Query     string
	Documents []*SearchResult
	Context   string
	Answer    string
	Metadata  map[string]interface{}
}

// NewRAGPipeline creates a new RAG pipeline
func NewRAGPipeline(knowledgeBase Knowledge, numDocuments int) *RAGPipeline {
	return &RAGPipeline{
		KnowledgeBase:    knowledgeBase,
		NumDocuments:     numDocuments,
		MaxContextLength: 2000, // Default max context length
	}
}

// Query executes a RAG query
func (r *RAGPipeline) Query(ctx context.Context, query string) (*RAGResult, error) {
	// 1. Retrieve relevant documents
	// If we have a reranker, we might want to retrieve more documents initially
	searchNum := r.NumDocuments
	if r.Reranker != nil {
		searchNum = r.NumDocuments * 2 // Retrieve twice as many for reranking
	}

	docs, err := r.KnowledgeBase.Search(ctx, query, searchNum)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge base: %w", err)
	}

	// 2. Rerank documents if a reranker is provided
	if r.Reranker != nil && len(docs) > 0 {
		rerankedDocs, err := r.Reranker.Rerank(ctx, query, docs)
		if err != nil {
			// Log error but continue with original docs? Or return error?
			// For now, return error as it's a pipeline failure
			return nil, fmt.Errorf("failed to rerank documents: %w", err)
		}
		docs = rerankedDocs
		// Limit to requested NumDocuments after reranking
		if len(docs) > r.NumDocuments {
			docs = docs[:r.NumDocuments]
		}
	}

	// 2. Build context from documents
	context := r.buildContext(docs)

	// 3. Create RAG result
	result := &RAGResult{
		Query:     query,
		Documents: docs,
		Context:   context,
		Metadata: map[string]interface{}{
			"num_documents":  len(docs),
			"context_length": len(context),
		},
	}

	return result, nil
}

// buildContext creates a context string from search results
func (r *RAGPipeline) buildContext(docs []*SearchResult) string {
	if len(docs) == 0 {
		return ""
	}

	var context strings.Builder
	context.WriteString("Relevant information:\n\n")

	for i, doc := range docs {
		// Truncate document content if too long
		content := doc.Document.Content
		if len(content) > r.MaxContextLength {
			content = content[:r.MaxContextLength] + "..."
		}

		context.WriteString(fmt.Sprintf("Document %d (Score: %.2f):\n%s\n\n",
			i+1, doc.Score, content))
	}

	return strings.TrimSpace(context.String())
}

// ScoreDocuments scores documents based on relevance
func (r *RAGPipeline) ScoreDocuments(docs []*SearchResult) []*SearchResult {
	// In a more advanced implementation, this could use:
	// - Semantic similarity scoring
	// - Recency weighting
	// - Source credibility weighting
	// - Custom ranking algorithms

	// For now, we'll return the documents as-is since they're already scored by the vector database
	return docs
}

// FilterDocuments filters documents based on relevance threshold
func (r *RAGPipeline) FilterDocuments(docs []*SearchResult, minScore float64) []*SearchResult {
	var filtered []*SearchResult
	for _, doc := range docs {
		if doc.Score >= minScore {
			filtered = append(filtered, doc)
		}
	}
	return filtered
}

// FormatDocument formats a document for use in context
func (r *RAGPipeline) FormatDocument(doc *document.Document) string {
	var builder strings.Builder

	if doc.Name != "" {
		builder.WriteString(fmt.Sprintf("Title: %s\n", doc.Name))
	}

	if source, ok := doc.Metadata["source"]; ok {
		builder.WriteString(fmt.Sprintf("Source: %s\n", source))
	}

	builder.WriteString(fmt.Sprintf("Content: %s\n", doc.Content))

	return builder.String()
}

// SetMaxContextLength sets the maximum context length for RAG operations
func (r *RAGPipeline) SetMaxContextLength(length int) {
	r.MaxContextLength = length
}

// SetNumDocuments sets the number of documents to retrieve
func (r *RAGPipeline) SetNumDocuments(num int) {
	r.NumDocuments = num
}

// SetReranker sets the reranker for the pipeline
func (r *RAGPipeline) SetReranker(reranker Reranker) {
	r.Reranker = reranker
}

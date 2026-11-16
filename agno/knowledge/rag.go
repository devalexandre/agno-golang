package knowledge

import (
	"context"
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/knowledge/querytransformer"
	"github.com/devalexandre/agno-golang/agno/knowledge/reranker"
)

// Reranker is the legacy reranking interface (no limit argument).
//
// Deprecated: implement reranker.Reranker from
// github.com/devalexandre/agno-golang/agno/knowledge/reranker and set it on
// RAGPipeline.RetrievalReranker or BaseKnowledge.Reranker. Legacy rerankers
// can be wrapped with AdaptLegacyReranker.
type Reranker interface {
	Rerank(ctx context.Context, query string, results []*SearchResult) ([]*SearchResult, error)
}

// RAGPipeline implements a Retrieval-Augmented Generation pipeline
type RAGPipeline struct {
	KnowledgeBase    Knowledge
	NumDocuments     int
	MaxContextLength int
	// Reranker is the legacy reranker.
	//
	// Deprecated: use RetrievalReranker. Ignored when RetrievalReranker is set.
	Reranker Reranker
	// RetrievalReranker reorders the candidates: the pipeline fetches
	// reranker.SearchLimit(NumDocuments) candidates (x3, capped at 100 by
	// default), reranks against the original query and keeps NumDocuments.
	RetrievalReranker reranker.Reranker
	// QueryTransformer optionally rewrites the query before searching
	// (e.g. querytransformer.HyDE).
	QueryTransformer querytransformer.QueryTransformer
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
	// 1. Retrieve (transform -> widened search -> rerank -> trim)
	rr := r.RetrievalReranker
	if rr == nil && r.Reranker != nil {
		rr = AdaptLegacyReranker(r.Reranker)
	}
	search := func(ctx context.Context, q string, limit int) ([]*SearchResult, error) {
		return r.KnowledgeBase.Search(ctx, q, limit)
	}
	var emb embedder.Embedder
	if eg, ok := r.KnowledgeBase.(interface{ GetEmbedder() embedder.Embedder }); ok && rr != nil {
		emb = eg.GetEmbedder()
	}
	docs, err := retrieve(ctx, search, query, r.NumDocuments, rr, r.QueryTransformer, emb)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge base: %w", err)
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

// SetReranker sets the legacy reranker for the pipeline.
//
// Deprecated: use SetRetrievalReranker.
func (r *RAGPipeline) SetReranker(legacy Reranker) {
	r.Reranker = legacy
}

// SetRetrievalReranker sets the canonical reranker for the pipeline.
func (r *RAGPipeline) SetRetrievalReranker(rr reranker.Reranker) {
	r.RetrievalReranker = rr
}

// SetQueryTransformer sets the query transformer for the pipeline.
func (r *RAGPipeline) SetQueryTransformer(qt querytransformer.QueryTransformer) {
	r.QueryTransformer = qt
}

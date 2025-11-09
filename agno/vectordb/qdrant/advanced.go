package qdrant

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/qdrant/go-client/qdrant"
)

// AdvancedFilter represents advanced filtering options for Qdrant
type AdvancedFilter struct {
	Must    []FilterCondition
	Should  []FilterCondition
	MustNot []FilterCondition
}

// FilterCondition represents a single filter condition
type FilterCondition struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
}

// FilterOperator represents filter operators
type FilterOperator string

const (
	FilterOpEqual              FilterOperator = "eq"
	FilterOpNotEqual           FilterOperator = "ne"
	FilterOpGreaterThan        FilterOperator = "gt"
	FilterOpGreaterThanOrEqual FilterOperator = "gte"
	FilterOpLessThan           FilterOperator = "lt"
	FilterOpLessThanOrEqual    FilterOperator = "lte"
	FilterOpIn                 FilterOperator = "in"
	FilterOpNotIn              FilterOperator = "nin"
	FilterOpContains           FilterOperator = "contains"
	FilterOpRange              FilterOperator = "range"
)

// RerankingConfig represents reranking configuration
type RerankingConfig struct {
	Enabled    bool
	Model      string // "cross-encoder" or "colbert"
	TopK       int    // Number of results to rerank
	ScoreBoost float64
}

// SearchWithAdvancedFilters performs search with advanced filtering
func (q *Qdrant) SearchWithAdvancedFilters(ctx context.Context, query string, limit int, advFilter *AdvancedFilter) ([]*vectordb.SearchResult, error) {
	// Generate query embedding
	queryEmbedding, err := q.EmbedQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if queryEmbedding == nil {
		return nil, fmt.Errorf("no query embedding generated")
	}

	// Create Qdrant filter from advanced filter
	filter := q.createAdvancedQdrantFilter(advFilter)

	// Perform search
	result, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: q.collection,
		Query: &qdrant.Query{
			Variant: &qdrant.Query_Nearest{
				Nearest: &qdrant.VectorInput{
					Variant: &qdrant.VectorInput_Dense{
						Dense: &qdrant.DenseVector{
							Data: convertToFloat32(queryEmbedding),
						},
					},
				},
			},
		},
		Filter:      filter,
		Limit:       func() *uint64 { l := uint64(limit); return &l }(),
		WithPayload: &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search points: %w", err)
	}

	// Convert to standard search results
	var results []*vectordb.SearchResult
	for _, point := range result {
		doc, err := q.payloadToDocument(point.Payload)
		if err != nil {
			continue
		}

		if doc.ID == "" {
			doc.ID = pointIDToString(point.Id)
		}

		distance := 1.0 - float64(point.Score)
		if q.Distance == vectordb.DistanceMaxInnerProduct || q.Distance == vectordb.DistanceDot {
			distance = -float64(point.Score)
		}

		searchResult := &vectordb.SearchResult{
			Document: doc,
			Score:    float64(point.Score),
			Distance: distance,
		}

		results = append(results, searchResult)
	}

	return results, nil
}

// SearchWithReranking performs search with reranking
func (q *Qdrant) SearchWithReranking(ctx context.Context, query string, limit int, filters map[string]interface{}, config *RerankingConfig) ([]*vectordb.SearchResult, error) {
	if config == nil || !config.Enabled {
		return q.Search(ctx, query, limit, filters)
	}

	// Get more results than needed for reranking
	fetchLimit := limit * 3
	if config.TopK > 0 && config.TopK > limit {
		fetchLimit = config.TopK
	}

	// Perform initial search
	results, err := q.Search(ctx, query, fetchLimit, filters)
	if err != nil {
		return nil, err
	}

	// Rerank results
	rerankedResults := q.rerankResults(query, results, config)

	// Limit to requested number
	if len(rerankedResults) > limit {
		rerankedResults = rerankedResults[:limit]
	}

	return rerankedResults, nil
}

// BatchSearch performs batch search operations
func (q *Qdrant) BatchSearch(ctx context.Context, queries []string, limit int, filters map[string]interface{}) ([][]*vectordb.SearchResult, error) {
	results := make([][]*vectordb.SearchResult, len(queries))

	for i, query := range queries {
		searchResults, err := q.Search(ctx, query, limit, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to search query %d: %w", i, err)
		}
		results[i] = searchResults
	}

	return results, nil
}

// BatchUpsert performs batch upsert operations with better performance
func (q *Qdrant) BatchUpsert(ctx context.Context, documents []*document.Document, batchSize int, filters map[string]interface{}) error {
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}

	// Ensure collection exists
	exists, err := q.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}
	if !exists {
		if err := q.Create(ctx); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
	}

	// Generate embeddings for all documents
	if err := q.EmbedDocuments(documents); err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	// Process in batches
	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[i:end]
		if err := q.Upsert(ctx, batch, filters); err != nil {
			return fmt.Errorf("failed to upsert batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// DeleteByFilter deletes documents matching the filter
func (q *Qdrant) DeleteByFilter(ctx context.Context, filters map[string]interface{}) error {
	if len(filters) == 0 {
		return fmt.Errorf("filters cannot be empty for delete operation")
	}

	filter := createQdrantFilter(filters)

	_, err := q.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: q.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: filter,
			},
		},
	})

	return err
}

// UpdatePayload updates payload for documents matching the filter
func (q *Qdrant) UpdatePayload(ctx context.Context, filters map[string]interface{}, payload map[string]interface{}) error {
	if len(filters) == 0 {
		return fmt.Errorf("filters cannot be empty for update operation")
	}

	filter := createQdrantFilter(filters)

	// Convert payload to Qdrant format
	qdrantPayload := make(map[string]*qdrant.Value)
	for k, v := range payload {
		qdrantPayload[k] = convertToQdrantValue(v)
	}

	_, err := q.client.SetPayload(ctx, &qdrant.SetPayloadPoints{
		CollectionName: q.collection,
		Payload:        qdrantPayload,
		PointsSelector: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: filter,
			},
		},
	})

	return err
}

// GetCollectionInfo returns information about the collection
func (q *Qdrant) GetCollectionInfo(ctx context.Context) (*CollectionInfo, error) {
	// Get collection count as a basic info
	count, err := q.GetCount(ctx)
	if err != nil {
		return nil, err
	}

	return &CollectionInfo{
		Name:        q.collection,
		VectorSize:  q.Dimensions,
		PointsCount: count,
		Status:      "active",
	}, nil
}

// CollectionInfo represents collection information
type CollectionInfo struct {
	Name        string
	VectorSize  int
	PointsCount int64
	Status      string
}

// Helper methods

// createAdvancedQdrantFilter creates a Qdrant filter from AdvancedFilter
func (q *Qdrant) createAdvancedQdrantFilter(advFilter *AdvancedFilter) *qdrant.Filter {
	if advFilter == nil {
		return nil
	}

	filter := &qdrant.Filter{}

	// Process Must conditions
	if len(advFilter.Must) > 0 {
		filter.Must = q.convertConditions(advFilter.Must)
	}

	// Process Should conditions
	if len(advFilter.Should) > 0 {
		filter.Should = q.convertConditions(advFilter.Should)
	}

	// Process MustNot conditions
	if len(advFilter.MustNot) > 0 {
		filter.MustNot = q.convertConditions(advFilter.MustNot)
	}

	return filter
}

// convertConditions converts FilterConditions to Qdrant conditions
func (q *Qdrant) convertConditions(conditions []FilterCondition) []*qdrant.Condition {
	var qdrantConditions []*qdrant.Condition

	for _, cond := range conditions {
		qdrantCond := q.convertCondition(cond)
		if qdrantCond != nil {
			qdrantConditions = append(qdrantConditions, qdrantCond)
		}
	}

	return qdrantConditions
}

// convertCondition converts a single FilterCondition to Qdrant condition
func (q *Qdrant) convertCondition(cond FilterCondition) *qdrant.Condition {
	switch cond.Operator {
	case FilterOpEqual:
		return &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key:   cond.Field,
					Match: createQdrantMatch(cond.Value),
				},
			},
		}

	case FilterOpRange:
		if rangeVal, ok := cond.Value.(map[string]interface{}); ok {
			var gte, lte *float64
			if v, ok := rangeVal["gte"].(float64); ok {
				gte = &v
			}
			if v, ok := rangeVal["lte"].(float64); ok {
				lte = &v
			}

			return &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: cond.Field,
						Range: &qdrant.Range{
							Gte: gte,
							Lte: lte,
						},
					},
				},
			}
		}

	case FilterOpIn:
		// Handle array values
		if values, ok := cond.Value.([]interface{}); ok {
			var conditions []*qdrant.Condition
			for _, v := range values {
				conditions = append(conditions, &qdrant.Condition{
					ConditionOneOf: &qdrant.Condition_Field{
						Field: &qdrant.FieldCondition{
							Key:   cond.Field,
							Match: createQdrantMatch(v),
						},
					},
				})
			}
			// Return as Should condition (OR logic)
			return &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Filter{
					Filter: &qdrant.Filter{
						Should: conditions,
					},
				},
			}
		}
	}

	return nil
}

// rerankResults reranks search results based on configuration
func (q *Qdrant) rerankResults(query string, results []*vectordb.SearchResult, config *RerankingConfig) []*vectordb.SearchResult {
	if len(results) == 0 {
		return results
	}

	// Simple reranking based on content similarity
	// In production, you would use a cross-encoder model here
	for _, result := range results {
		// Calculate additional score based on query-content similarity
		contentScore := q.calculateContentSimilarity(query, result.Document.Content)

		// Boost the score
		boost := config.ScoreBoost
		if boost == 0 {
			boost = 1.2
		}

		result.Score = result.Score*0.7 + contentScore*0.3*boost
	}

	// Sort by new score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// calculateContentSimilarity calculates simple content similarity
func (q *Qdrant) calculateContentSimilarity(query, content string) float64 {
	// Simple word overlap similarity
	queryWords := q.tokenize(query)
	contentWords := q.tokenize(content)

	if len(queryWords) == 0 || len(contentWords) == 0 {
		return 0.0
	}

	// Count overlapping words
	overlap := 0
	querySet := make(map[string]bool)
	for _, word := range queryWords {
		querySet[word] = true
	}

	for _, word := range contentWords {
		if querySet[word] {
			overlap++
		}
	}

	// Calculate Jaccard similarity
	union := len(queryWords) + len(contentWords) - overlap
	if union == 0 {
		return 0.0
	}

	return float64(overlap) / float64(union)
}

// tokenize splits text into words
func (q *Qdrant) tokenize(text string) []string {
	// Simple tokenization - split by spaces and convert to lowercase
	words := []string{}
	current := ""

	for _, char := range text {
		if char == ' ' || char == '\n' || char == '\t' {
			if current != "" {
				words = append(words, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		words = append(words, current)
	}

	return words
}

// SparseVector represents a sparse vector for hybrid search
type SparseVector struct {
	Indices []uint32
	Values  []float32
}

// HybridSearchWithSparse performs hybrid search using dense and sparse vectors
func (q *Qdrant) HybridSearchWithSparse(ctx context.Context, query string, sparseVector *SparseVector, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Generate dense query embedding
	queryEmbedding, err := q.EmbedQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if queryEmbedding == nil {
		return nil, fmt.Errorf("no query embedding generated")
	}

	// Create filter if provided
	var filter *qdrant.Filter
	if filters != nil && len(filters) > 0 {
		filter = createQdrantFilter(filters)
	}

	// Perform dense search
	denseResults, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: q.collection,
		Query: &qdrant.Query{
			Variant: &qdrant.Query_Nearest{
				Nearest: &qdrant.VectorInput{
					Variant: &qdrant.VectorInput_Dense{
						Dense: &qdrant.DenseVector{
							Data: convertToFloat32(queryEmbedding),
						},
					},
				},
			},
		},
		Filter:      filter,
		Limit:       func() *uint64 { l := uint64(limit * 2); return &l }(),
		WithPayload: &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to perform dense search: %w", err)
	}

	// Convert to search results
	var results []*vectordb.SearchResult
	for _, point := range denseResults {
		doc, err := q.payloadToDocument(point.Payload)
		if err != nil {
			continue
		}

		if doc.ID == "" {
			doc.ID = pointIDToString(point.Id)
		}

		distance := 1.0 - float64(point.Score)
		searchResult := &vectordb.SearchResult{
			Document: doc,
			Score:    float64(point.Score),
			Distance: distance,
		}

		results = append(results, searchResult)
	}

	// If sparse vector provided, combine with keyword search
	if sparseVector != nil {
		keywordResults, err := q.KeywordSearch(ctx, query, limit*2, filters)
		if err == nil {
			// Merge results
			results = q.mergeSearchResults(results, keywordResults, limit)
		}
	}

	return results, nil
}

// mergeSearchResults merges two sets of search results
func (q *Qdrant) mergeSearchResults(results1, results2 []*vectordb.SearchResult, limit int) []*vectordb.SearchResult {
	resultMap := make(map[string]*vectordb.SearchResult)

	// Add first set with weight
	for i, result := range results1 {
		score := 1.0 - (float64(i) / float64(len(results1)))
		result.Score = score * 0.6 // 60% weight
		resultMap[result.Document.ID] = result
	}

	// Add second set with weight
	for i, result := range results2 {
		score := 1.0 - (float64(i) / float64(len(results2)))
		if existing, exists := resultMap[result.Document.ID]; exists {
			existing.Score += score * 0.4 // 40% weight
		} else {
			result.Score = score * 0.4
			resultMap[result.Document.ID] = result
		}
	}

	// Convert to slice and sort
	var merged []*vectordb.SearchResult
	for _, result := range resultMap {
		merged = append(merged, result)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	// Limit results
	if len(merged) > limit {
		merged = merged[:limit]
	}

	return merged
}

// CalculateRelevanceScore calculates relevance score for a document
func (q *Qdrant) CalculateRelevanceScore(query string, doc *document.Document) float64 {
	if doc == nil {
		return 0.0
	}

	// Calculate multiple relevance signals
	contentSim := q.calculateContentSimilarity(query, doc.Content)

	// Title/name similarity (if available)
	nameSim := 0.0
	if doc.Name != "" {
		nameSim = q.calculateContentSimilarity(query, doc.Name)
	}

	// Combine scores
	score := contentSim*0.7 + nameSim*0.3

	return math.Min(1.0, score)
}

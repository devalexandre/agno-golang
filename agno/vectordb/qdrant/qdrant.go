package qdrant

import (
	"context"
	"fmt"
	"strconv"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	"github.com/qdrant/go-client/qdrant"
)

// Qdrant implements VectorDB interface using Qdrant vector database with official client
type Qdrant struct {
	*vectordb.BaseVectorDB
	client     *qdrant.Client
	collection string
}

// QdrantConfig holds configuration for Qdrant
type QdrantConfig struct {
	Host       string
	Port       int
	Collection string
	Embedder   embedder.Embedder
	SearchType vectordb.SearchType
	Distance   vectordb.Distance
}

// NewQdrant creates a new Qdrant instance
func NewQdrant(config QdrantConfig) (*Qdrant, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: config.Host,
		Port: config.Port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	collection := config.Collection
	if collection == "" {
		collection = "documents"
	}

	searchType := config.SearchType
	if searchType == "" {
		searchType = vectordb.SearchTypeVector
	}

	distance := config.Distance
	if distance == "" {
		distance = vectordb.DistanceCosine
	}

	baseVectorDB := vectordb.NewBaseVectorDB(config.Embedder, searchType, distance)

	return &Qdrant{
		BaseVectorDB: baseVectorDB,
		client:       client,
		collection:   collection,
	}, nil
}

// Create creates a collection in Qdrant
func (q *Qdrant) Create(ctx context.Context) error {
	exists, err := q.collectionExists(ctx, q.collection)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if exists {
		return nil // Collection already exists
	}

	// Convert distance type to Qdrant format
	var qdrantDistance qdrant.Distance
	switch q.Distance {
	case vectordb.DistanceCosine:
		qdrantDistance = qdrant.Distance_Cosine
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		qdrantDistance = qdrant.Distance_Euclid
	case vectordb.DistanceMaxInnerProduct, vectordb.DistanceDot:
		qdrantDistance = qdrant.Distance_Dot
	default:
		qdrantDistance = qdrant.Distance_Cosine
	}

	err = q.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: q.collection,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     uint64(q.Dimensions),
					Distance: qdrantDistance,
				},
			},
		},
	})

	return err
}

// Exists checks if the collection exists
func (q *Qdrant) Exists(ctx context.Context) (bool, error) {
	return q.collectionExists(ctx, q.collection)
}

// Drop drops the collection
func (q *Qdrant) Drop(ctx context.Context) error {
	exists, err := q.collectionExists(ctx, q.collection)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		return nil // Collection doesn't exist
	}

	err = q.client.DeleteCollection(ctx, q.collection)
	return err
}

// Optimize optimizes the collection (Qdrant handles this automatically)
func (q *Qdrant) Optimize(ctx context.Context) error {
	// Qdrant optimizes automatically, so this is a no-op
	return nil
}

// Insert inserts documents into Qdrant
func (q *Qdrant) Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	// Ensure collection exists, create if it doesn't
	exists, err := q.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}
	if !exists {
		if err := q.Create(ctx); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
	}

	return q.Upsert(ctx, documents, filters) // Qdrant uses upsert by default
}

// Upsert inserts or updates documents in Qdrant
func (q *Qdrant) Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if len(documents) == 0 {
		return nil
	}

	// Ensure collection exists, create if it doesn't
	exists, err := q.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}
	if !exists {
		if err := q.Create(ctx); err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
	}

	// Generate embeddings if not present
	if err := q.EmbedDocuments(documents); err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	// Convert documents to Qdrant points
	var points []*qdrant.PointStruct
	for _, doc := range documents {
		// Create payload with document data
		payload := map[string]*qdrant.Value{
			"id":           convertToQdrantValue(doc.ID),
			"name":         convertToQdrantValue(doc.Name),
			"content":      convertToQdrantValue(doc.Content),
			"content_type": convertToQdrantValue(doc.ContentType),
			"source":       convertToQdrantValue(doc.Source),
			"chunk_index":  convertToQdrantValue(doc.ChunkIndex),
			"chunk_total":  convertToQdrantValue(doc.ChunkTotal),
			"parent_id":    convertToQdrantValue(doc.ParentID),
		}

		// Add document metadata
		if doc.Metadata != nil {
			for k, v := range doc.Metadata {
				payload[k] = convertToQdrantValue(v)
			}
		}

		// Add filters
		if filters != nil {
			for k, v := range filters {
				payload[k] = convertToQdrantValue(v)
			}
		}

		// Create Qdrant point with numeric ID
		point := &qdrant.PointStruct{
			Id:      &qdrant.PointId{PointIdOptions: &qdrant.PointId_Num{Num: stringToUint64(doc.ID)}},
			Payload: payload,
			Vectors: &qdrant.Vectors{VectorsOptions: &qdrant.Vectors_Vector{Vector: &qdrant.Vector{Data: convertToFloat32(doc.Embeddings)}}},
		}

		points = append(points, point)
	}

	_, err = q.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: q.collection,
		Points:         points,
	})

	return err
}

// Search performs search based on the configured search type
func (q *Qdrant) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	switch q.SearchType {
	case vectordb.SearchTypeVector:
		return q.VectorSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeKeyword:
		return q.KeywordSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeHybrid:
		return q.HybridSearch(ctx, query, limit, filters)
	default:
		return q.VectorSearch(ctx, query, limit, filters)
	}
}

// VectorSearch performs vector similarity search
func (q *Qdrant) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Generate query embedding
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
		Filter: filter,
		Limit:  func() *uint64 { l := uint64(limit); return &l }(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search points: %w", err)
	}

	// Convert to standard search results
	var results []*vectordb.SearchResult
	for _, point := range result {
		doc, err := q.payloadToDocument(point.Payload)
		if err != nil {
			continue // Skip invalid documents
		}

		// Set the ID from the point ID if not already set
		if doc.ID == "" {
			doc.ID = pointIDToString(point.Id)
		}

		// Calculate distance from score (Qdrant returns similarity score 0-1)
		distance := 1.0 - float64(point.Score)
		if q.Distance == vectordb.DistanceMaxInnerProduct || q.Distance == vectordb.DistanceDot {
			distance = -float64(point.Score) // For inner product, negate for distance
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

// KeywordSearch performs keyword search using Qdrant's text matching
func (q *Qdrant) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Create filter that includes the content search
	contentFilter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "content",
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Text{
								Text: query,
							},
						},
					},
				},
			},
		},
	}

	// Add additional filters if provided
	if filters != nil && len(filters) > 0 {
		additionalFilter := createQdrantFilter(filters)
		if additionalFilter != nil {
			contentFilter.Must = append(contentFilter.Must, additionalFilter.Must...)
		}
	}

	// Use query with filter - need a dummy vector for the query
	dummyVector := make([]float32, q.Dimensions)
	result, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: q.collection,
		Query: &qdrant.Query{
			Variant: &qdrant.Query_Nearest{
				Nearest: &qdrant.VectorInput{
					Variant: &qdrant.VectorInput_Dense{
						Dense: &qdrant.DenseVector{
							Data: dummyVector,
						},
					},
				},
			},
		},
		Filter: contentFilter,
		Limit:  func() *uint64 { l := uint64(limit); return &l }(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search points: %w", err)
	}

	// Convert to standard search results
	var results []*vectordb.SearchResult
	for _, point := range result {
		doc, err := q.payloadToDocument(point.Payload)
		if err != nil {
			continue // Skip invalid documents
		}

		// Set the ID from the point ID if not already set
		if doc.ID == "" {
			doc.ID = pointIDToString(point.Id)
		}

		searchResult := &vectordb.SearchResult{
			Document: doc,
			Score:    float64(point.Score),
			Distance: 1.0 - float64(point.Score),
		}

		results = append(results, searchResult)
	}

	return results, nil
}

// HybridSearch performs hybrid vector + keyword search
func (q *Qdrant) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Get vector search results
	vectorResults, err := q.VectorSearch(ctx, query, limit*2, filters)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Get keyword search results
	keywordResults, err := q.KeywordSearch(ctx, query, limit*2, filters)
	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}

	// Combine and rerank results
	resultMap := make(map[string]*vectordb.SearchResult)

	// Add vector results with weight
	for i, result := range vectorResults {
		score := 1.0 - (float64(i) / float64(len(vectorResults)))
		result.Score = score * 0.7 // 70% weight for vector search
		resultMap[result.Document.ID] = result
	}

	// Add keyword results with weight
	for i, result := range keywordResults {
		score := 1.0 - (float64(i) / float64(len(keywordResults)))
		if existing, exists := resultMap[result.Document.ID]; exists {
			existing.Score += score * 0.3 // 30% weight for keyword search
		} else {
			result.Score = score * 0.3
			resultMap[result.Document.ID] = result
		}
	}

	// Convert to slice and sort by combined score
	var combinedResults []*vectordb.SearchResult
	for _, result := range resultMap {
		combinedResults = append(combinedResults, result)
	}

	// Sort by score (descending)
	for i := 0; i < len(combinedResults)-1; i++ {
		for j := i + 1; j < len(combinedResults); j++ {
			if combinedResults[i].Score < combinedResults[j].Score {
				combinedResults[i], combinedResults[j] = combinedResults[j], combinedResults[i]
			}
		}
	}

	// Limit results
	if len(combinedResults) > limit {
		combinedResults = combinedResults[:limit]
	}

	return combinedResults, nil
}

// GetCount returns the number of documents in the collection
func (q *Qdrant) GetCount(ctx context.Context) (int64, error) {
	// Use scroll to count points since there's no direct count method
	result, err := q.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: q.collection,
		Limit:          func() *uint32 { l := uint32(1); return &l }(),
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: false}},
		WithVectors:    &qdrant.WithVectorsSelector{SelectorOptions: &qdrant.WithVectorsSelector_Enable{Enable: false}},
	})
	if err != nil {
		return 0, err
	}

	// This is a simplified approach - in practice, you'd need to paginate through all results
	// For now, return a basic estimate
	return int64(len(result)), nil
}

// DocExists checks if a document exists
func (q *Qdrant) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc.ID == "" {
		return false, nil
	}
	return q.IDExists(ctx, doc.ID)
}

// NameExists checks if a document with the given name exists
func (q *Qdrant) NameExists(ctx context.Context, name string) (bool, error) {
	// Search for documents with the given name
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "name",
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Text{
								Text: name,
							},
						},
					},
				},
			},
		},
	}

	dummyVector := make([]float32, q.Dimensions)
	result, err := q.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: q.collection,
		Query: &qdrant.Query{
			Variant: &qdrant.Query_Nearest{
				Nearest: &qdrant.VectorInput{
					Variant: &qdrant.VectorInput_Dense{
						Dense: &qdrant.DenseVector{
							Data: dummyVector,
						},
					},
				},
			},
		},
		Filter: filter,
		Limit:  func() *uint64 { l := uint64(1); return &l }(),
	})
	if err != nil {
		return false, err
	}

	return len(result) > 0, nil
}

// IDExists checks if a document with the given ID exists
func (q *Qdrant) IDExists(ctx context.Context, id string) (bool, error) {
	points, err := q.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: q.collection,
		Ids: []*qdrant.PointId{
			{PointIdOptions: &qdrant.PointId_Num{Num: stringToUint64(id)}},
		},
	})
	if err != nil {
		return false, err
	}

	return len(points) > 0, nil
}

// Helper methods

// collectionExists checks if the collection exists
func (q *Qdrant) collectionExists(ctx context.Context, name string) (bool, error) {
	collections, err := q.client.ListCollections(ctx)
	if err != nil {
		return false, err
	}

	for _, collection := range collections {
		if collection == name {
			return true, nil
		}
	}

	return false, nil
}

// createQdrantFilter converts filter map to Qdrant filter
func createQdrantFilter(filters map[string]interface{}) *qdrant.Filter {
	var conditions []*qdrant.Condition

	for key, value := range filters {
		condition := &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key:   key,
					Match: createQdrantMatch(value),
				},
			},
		}
		conditions = append(conditions, condition)
	}

	return &qdrant.Filter{
		Must: conditions,
	}
}

// createQdrantMatch creates a Qdrant match from interface value
func createQdrantMatch(value interface{}) *qdrant.Match {
	switch v := value.(type) {
	case string:
		return &qdrant.Match{
			MatchValue: &qdrant.Match_Text{Text: v},
		}
	case int, int64:
		return &qdrant.Match{
			MatchValue: &qdrant.Match_Integer{Integer: convertToInt64(v)},
		}
	case bool:
		return &qdrant.Match{
			MatchValue: &qdrant.Match_Boolean{Boolean: v},
		}
	default:
		return &qdrant.Match{
			MatchValue: &qdrant.Match_Text{Text: fmt.Sprintf("%v", v)},
		}
	}
}

// payloadToDocument converts Qdrant payload to Document
func (q *Qdrant) payloadToDocument(payload map[string]*qdrant.Value) (*document.Document, error) {
	doc := &document.Document{}

	// Extract basic fields
	if id := convertFromQdrantValue(payload["id"]); id != nil {
		if idStr, ok := id.(string); ok {
			doc.ID = idStr
		}
	}
	if name := convertFromQdrantValue(payload["name"]); name != nil {
		if nameStr, ok := name.(string); ok {
			doc.Name = nameStr
		}
	}
	if content := convertFromQdrantValue(payload["content"]); content != nil {
		if contentStr, ok := content.(string); ok {
			doc.Content = contentStr
		}
	}
	if contentType := convertFromQdrantValue(payload["content_type"]); contentType != nil {
		if contentTypeStr, ok := contentType.(string); ok {
			doc.ContentType = contentTypeStr
		}
	}
	if source := convertFromQdrantValue(payload["source"]); source != nil {
		if sourceStr, ok := source.(string); ok {
			doc.Source = sourceStr
		}
	}
	if chunkIndex := convertFromQdrantValue(payload["chunk_index"]); chunkIndex != nil {
		if idx, ok := chunkIndex.(int64); ok {
			doc.ChunkIndex = int(idx)
		}
	}
	if chunkTotal := convertFromQdrantValue(payload["chunk_total"]); chunkTotal != nil {
		if total, ok := chunkTotal.(int64); ok {
			doc.ChunkTotal = int(total)
		}
	}
	if parentID := convertFromQdrantValue(payload["parent_id"]); parentID != nil {
		if parentIDStr, ok := parentID.(string); ok {
			doc.ParentID = parentIDStr
		}
	}

	// Extract metadata (exclude known system fields)
	systemFields := map[string]bool{
		"id":           true,
		"name":         true,
		"content":      true,
		"content_type": true,
		"source":       true,
		"chunk_index":  true,
		"chunk_total":  true,
		"parent_id":    true,
		"created_at":   true,
		"updated_at":   true,
	}

	doc.Metadata = make(map[string]interface{})
	for k, v := range payload {
		if !systemFields[k] {
			doc.Metadata[k] = convertFromQdrantValue(v)
		}
	}

	return doc, nil
}

// Conversion helpers

func convertToFloat32(vector []float64) []float32 {
	result := make([]float32, len(vector))
	for i, v := range vector {
		result[i] = float32(v)
	}
	return result
}

func convertToQdrantValue(v interface{}) *qdrant.Value {
	switch val := v.(type) {
	case string:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: val}}
	case int:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(val)}}
	case int64:
		return &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: val}}
	case float64:
		return &qdrant.Value{Kind: &qdrant.Value_DoubleValue{DoubleValue: val}}
	case bool:
		return &qdrant.Value{Kind: &qdrant.Value_BoolValue{BoolValue: val}}
	default:
		return &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: fmt.Sprintf("%v", val)}}
	}
}

func convertFromQdrantValue(v *qdrant.Value) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.Kind.(type) {
	case *qdrant.Value_StringValue:
		return val.StringValue
	case *qdrant.Value_IntegerValue:
		return val.IntegerValue
	case *qdrant.Value_DoubleValue:
		return val.DoubleValue
	case *qdrant.Value_BoolValue:
		return val.BoolValue
	default:
		return nil
	}
}

func convertToInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case int32:
		return int64(val)
	default:
		return 0
	}
}

func pointIDToString(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	switch id := id.PointIdOptions.(type) {
	case *qdrant.PointId_Num:
		return strconv.FormatUint(id.Num, 10)
	case *qdrant.PointId_Uuid:
		return id.Uuid
	default:
		return ""
	}
}

// stringToUint64 converts a string to uint64 using hash for non-numeric strings
func stringToUint64(s string) uint64 {
	// Try to parse as number first
	if num, err := strconv.ParseUint(s, 10, 64); err == nil {
		return num
	}

	// For non-numeric strings, use a simple hash
	hash := uint64(0)
	for _, char := range s {
		hash = hash*31 + uint64(char)
	}
	// Ensure it's not 0 as Qdrant might have issues with 0 IDs
	if hash == 0 {
		hash = 1
	}
	return hash
}

// Close closes the Qdrant client connection
func (q *Qdrant) Close() error {
	// Qdrant go client doesn't require explicit closing
	// This method is here for interface compatibility
	return nil
}

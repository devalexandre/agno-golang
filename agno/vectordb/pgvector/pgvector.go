package pgvector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
	_ "github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

// PgVector implements VectorDB interface using PostgreSQL with pgvector extension
type PgVector struct {
	*vectordb.BaseVectorDB
	db         *sql.DB
	tableName  string
	schema     string
	dimensions int
}

// PgVectorConfig holds configuration for PgVector
type PgVectorConfig struct {
	ConnectionString string
	TableName        string
	Schema           string
	Embedder         embedder.Embedder
	SearchType       vectordb.SearchType
	Distance         vectordb.Distance
}

// NewPgVector creates a new PgVector instance
func NewPgVector(config PgVectorConfig) (*PgVector, error) {
	db, err := sql.Open("postgres", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	tableName := config.TableName
	if tableName == "" {
		tableName = "documents"
	}

	schema := config.Schema
	if schema == "" {
		schema = "public"
	}

	searchType := config.SearchType
	if searchType == "" {
		searchType = vectordb.SearchTypeVector
	}

	distance := config.Distance
	if distance == "" {
		distance = vectordb.DistanceCosine
	}

	dimensions := 0
	if config.Embedder != nil {
		dimensions = config.Embedder.GetDimensions()
	}

	baseVectorDB := vectordb.NewBaseVectorDB(config.Embedder, searchType, distance)

	return &PgVector{
		BaseVectorDB: baseVectorDB,
		db:           db,
		tableName:    tableName,
		schema:       schema,
		dimensions:   dimensions,
	}, nil
}

// convertFloat64ToFloat32 converts []float64 to []float32 for pgvector compatibility
func convertFloat64ToFloat32(input []float64) []float32 {
	if input == nil {
		return nil
	}
	output := make([]float32, len(input))
	for i, v := range input {
		output[i] = float32(v)
	}
	return output
}

// convertFloat32ToFloat64 converts []float32 to []float64 for compatibility
func convertFloat32ToFloat64(input []float32) []float64 {
	if input == nil {
		return nil
	}
	output := make([]float64, len(input))
	for i, v := range input {
		output[i] = float64(v)
	}
	return output
}

// Create creates the table and enables pgvector extension
func (p *PgVector) Create(ctx context.Context) error {
	// Create schema if it doesn't exist
	if p.schema != "public" {
		_, err := p.db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", p.schema))
		if err != nil {
			return fmt.Errorf("failed to create schema: %w", err)
		}
	}

	// Enable pgvector extension
	_, err := p.db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %w", err)
	}

	// Create table
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			id TEXT PRIMARY KEY,
			name TEXT,
			content TEXT NOT NULL,
			content_type TEXT DEFAULT 'text/plain',
			metadata JSONB,
			source TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			embeddings vector(%d),
			chunk_index INTEGER DEFAULT 0,
			chunk_total INTEGER DEFAULT 1,
			parent_id TEXT
		)
	`, p.schema, p.tableName, p.dimensions)

	_, err = p.db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indexes
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_id ON %s.%s (id)", p.tableName, p.schema, p.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_name ON %s.%s (name)", p.tableName, p.schema, p.tableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_content_gin ON %s.%s USING gin(to_tsvector('english', content))", p.tableName, p.schema, p.tableName),
	}

	// Add vector index based on distance type
	var vectorIndex string
	switch p.Distance {
	case vectordb.DistanceCosine:
		vectorIndex = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_embeddings_cosine ON %s.%s USING hnsw (embeddings vector_cosine_ops)", p.tableName, p.schema, p.tableName)
	case vectordb.DistanceL2:
		vectorIndex = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_embeddings_l2 ON %s.%s USING hnsw (embeddings vector_l2_ops)", p.tableName, p.schema, p.tableName)
	case vectordb.DistanceMaxInnerProduct:
		vectorIndex = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_embeddings_ip ON %s.%s USING hnsw (embeddings vector_ip_ops)", p.tableName, p.schema, p.tableName)
	default:
		vectorIndex = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_embeddings_cosine ON %s.%s USING hnsw (embeddings vector_cosine_ops)", p.tableName, p.schema, p.tableName)
	}

	indexes = append(indexes, vectorIndex)

	for _, indexSQL := range indexes {
		_, err = p.db.ExecContext(ctx, indexSQL)
		if err != nil {
			// Log but don't fail on index creation errors
			fmt.Printf("Warning: failed to create index: %v\n", err)
		}
	}

	return nil
}

// Exists checks if the table exists
func (p *PgVector) Exists(ctx context.Context) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = $1 
			AND table_name = $2
		)
	`
	var exists bool
	err := p.db.QueryRowContext(ctx, query, p.schema, p.tableName).Scan(&exists)
	return exists, err
}

// Drop drops the table
func (p *PgVector) Drop(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", p.schema, p.tableName))
	return err
}

// Optimize optimizes the table (placeholder for future implementation)
func (p *PgVector) Optimize(ctx context.Context) error {
	// VACUUM and ANALYZE
	_, err := p.db.ExecContext(ctx, fmt.Sprintf("VACUUM ANALYZE %s.%s", p.schema, p.tableName))
	return err
}

// Insert inserts documents into the database
func (p *PgVector) Insert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if len(documents) == 0 {
		return nil
	}

	// Ensure table exists, create if it doesn't
	exists, err := p.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}
	if !exists {
		if err := p.Create(ctx); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Generate embeddings if not present
	if err := p.EmbedDocuments(documents); err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	// Prepare insert statement
	insertSQL := fmt.Sprintf(`
		INSERT INTO %s.%s (id, name, content, content_type, metadata, source, embeddings, chunk_index, chunk_total, parent_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7::vector, $8, $9, $10)
	`, p.schema, p.tableName)

	stmt, err := p.db.PrepareContext(ctx, insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	for _, doc := range documents {
		// Merge filters with document metadata
		metadata := make(map[string]interface{})
		for k, v := range doc.Metadata {
			metadata[k] = v
		}
		for k, v := range filters {
			metadata[k] = v
		}

		// Convert metadata to JSON
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata for document %s: %w", doc.ID, err)
		}

		// Convert embedding to PostgreSQL vector format
		var embeddingStr interface{}
		if doc.Embeddings != nil {
			embeddingStr = pgvector.NewVector(convertFloat64ToFloat32(doc.Embeddings))
		} else {
			embeddingStr = nil
		}

		_, err = stmt.ExecContext(ctx,
			doc.ID,
			doc.Name,
			doc.Content,
			doc.ContentType,
			string(metadataJSON),
			doc.Source,
			embeddingStr,
			doc.ChunkIndex,
			doc.ChunkTotal,
			doc.ParentID,
		)
		if err != nil {
			return fmt.Errorf("failed to insert document %s: %w", doc.ID, err)
		}
	}

	return nil
}

// Upsert inserts or updates documents
func (p *PgVector) Upsert(ctx context.Context, documents []*document.Document, filters map[string]interface{}) error {
	if len(documents) == 0 {
		return nil
	}

	// Ensure table exists, create if it doesn't
	exists, err := p.Exists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %w", err)
	}
	if !exists {
		if err := p.Create(ctx); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Generate embeddings if not present
	if err := p.EmbedDocuments(documents); err != nil {
		return fmt.Errorf("failed to embed documents: %w", err)
	}

	// Prepare upsert statement
	upsertSQL := fmt.Sprintf(`
		INSERT INTO %s.%s (id, name, content, content_type, metadata, source, embeddings, chunk_index, chunk_total, parent_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::vector, $8, $9, $10, NOW())
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			content = EXCLUDED.content,
			content_type = EXCLUDED.content_type,
			metadata = EXCLUDED.metadata,
			source = EXCLUDED.source,
			embeddings = EXCLUDED.embeddings,
			chunk_index = EXCLUDED.chunk_index,
			chunk_total = EXCLUDED.chunk_total,
			parent_id = EXCLUDED.parent_id,
			updated_at = NOW()
	`, p.schema, p.tableName)

	stmt, err := p.db.PrepareContext(ctx, upsertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare upsert statement: %w", err)
	}
	defer stmt.Close()

	for _, doc := range documents {
		// Merge filters with document metadata
		metadata := make(map[string]interface{})
		for k, v := range doc.Metadata {
			metadata[k] = v
		}
		for k, v := range filters {
			metadata[k] = v
		}

		// Convert metadata to JSON
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata for document %s: %w", doc.ID, err)
		}

		// Convert embedding to PostgreSQL vector format
		var embeddingStr interface{}
		if doc.Embeddings != nil {
			embeddingStr = pgvector.NewVector(convertFloat64ToFloat32(doc.Embeddings))
		} else {
			embeddingStr = nil
		}

		_, err = stmt.ExecContext(ctx,
			doc.ID,
			doc.Name,
			doc.Content,
			doc.ContentType,
			string(metadataJSON),
			doc.Source,
			embeddingStr,
			doc.ChunkIndex,
			doc.ChunkTotal,
			doc.ParentID,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert document %s: %w", doc.ID, err)
		}
	}

	return nil
}

// Search performs search based on the configured search type
func (p *PgVector) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	switch p.SearchType {
	case vectordb.SearchTypeVector:
		return p.VectorSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeKeyword:
		return p.KeywordSearch(ctx, query, limit, filters)
	case vectordb.SearchTypeHybrid:
		return p.HybridSearch(ctx, query, limit, filters)
	default:
		return p.VectorSearch(ctx, query, limit, filters)
	}
}

// VectorSearch performs vector similarity search
func (p *PgVector) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Generate query embedding
	queryEmbedding, err := p.EmbedQuery(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	if queryEmbedding == nil {
		return nil, fmt.Errorf("no query embedding generated")
	}

	// Build WHERE clause for filters
	whereClause, args := p.buildWhereClause(filters, 2) // Start from $2 since $1 is the embedding

	// Choose distance operator based on distance type
	var distanceOp string
	var orderBy string
	switch p.Distance {
	case vectordb.DistanceCosine:
		distanceOp = "<=>"
		orderBy = "embeddings <=> $1"
	case vectordb.DistanceL2:
		distanceOp = "<->"
		orderBy = "embeddings <-> $1"
	case vectordb.DistanceMaxInnerProduct:
		distanceOp = "<#>"
		orderBy = "embeddings <#> $1 DESC"
	default:
		distanceOp = "<=>"
		orderBy = "embeddings <=> $1"
	}

	searchSQL := fmt.Sprintf(`
		SELECT id, name, content, content_type, metadata, source, created_at, updated_at, 
			   embeddings, chunk_index, chunk_total, parent_id,
			   embeddings %s $1 as distance
		FROM %s.%s
		%s
		ORDER BY %s
		LIMIT $%d
	`, distanceOp, p.schema, p.tableName, whereClause, orderBy, len(args)+2)

	// Prepare arguments
	queryArgs := []interface{}{pgvector.NewVector(convertFloat64ToFloat32(queryEmbedding))}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, limit)

	rows, err := p.db.QueryContext(ctx, searchSQL, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search: %w", err)
	}
	defer rows.Close()

	return p.scanSearchResults(rows)
}

// KeywordSearch performs full-text search
func (p *PgVector) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Build WHERE clause for filters
	whereClause, args := p.buildWhereClause(filters, 2) // Start from $2

	// Combine with full-text search condition
	if whereClause != "" {
		whereClause = whereClause + " AND to_tsvector('english', content) @@ websearch_to_tsquery('english', $1)"
	} else {
		whereClause = "WHERE to_tsvector('english', content) @@ websearch_to_tsquery('english', $1)"
	}

	searchSQL := fmt.Sprintf(`
		SELECT id, name, content, content_type, metadata, source, created_at, updated_at, 
			   embeddings, chunk_index, chunk_total, parent_id,
			   ts_rank_cd(to_tsvector('english', content), websearch_to_tsquery('english', $1)) as distance
		FROM %s.%s
		%s
		ORDER BY ts_rank_cd(to_tsvector('english', content), websearch_to_tsquery('english', $1)) DESC
		LIMIT $%d
	`, p.schema, p.tableName, whereClause, len(args)+2)

	// Prepare arguments
	queryArgs := []interface{}{query}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, limit)

	rows, err := p.db.QueryContext(ctx, searchSQL, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute keyword search: %w", err)
	}
	defer rows.Close()

	return p.scanSearchResults(rows)
}

// HybridSearch performs hybrid vector + keyword search
func (p *PgVector) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	// Get vector search results
	vectorResults, err := p.VectorSearch(ctx, query, limit*2, filters) // Get more results for reranking
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Get keyword search results
	keywordResults, err := p.KeywordSearch(ctx, query, limit*2, filters)
	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}

	// Combine and rerank results (simple implementation)
	resultMap := make(map[string]*vectordb.SearchResult)

	// Add vector results with weight
	for i, result := range vectorResults {
		score := 1.0 - (float64(i) / float64(len(vectorResults))) // Normalize rank to 0-1
		result.Score = score * 0.7                                // 70% weight for vector search
		resultMap[result.Document.ID] = result
	}

	// Add keyword results with weight
	for i, result := range keywordResults {
		score := 1.0 - (float64(i) / float64(len(keywordResults))) // Normalize rank to 0-1
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

// GetCount returns the number of documents in the database
func (p *PgVector) GetCount(ctx context.Context) (int64, error) {
	var count int64
	err := p.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", p.schema, p.tableName)).Scan(&count)
	return count, err
}

// DocExists checks if a document exists
func (p *PgVector) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc.ID == "" {
		return false, nil
	}
	return p.IDExists(ctx, doc.ID)
}

// NameExists checks if a document with the given name exists
func (p *PgVector) NameExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := p.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.%s WHERE name = $1)", p.schema, p.tableName),
		name).Scan(&exists)
	return exists, err
}

// IDExists checks if a document with the given ID exists
func (p *PgVector) IDExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := p.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.%s WHERE id = $1)", p.schema, p.tableName),
		id).Scan(&exists)
	return exists, err
}

// Helper methods

// buildWhereClause builds WHERE clause for filters
func (p *PgVector) buildWhereClause(filters map[string]interface{}, startIndex int) (string, []interface{}) {
	if len(filters) == 0 {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argIndex := startIndex

	for key, value := range filters {
		// Convert value to string for JSON metadata comparison
		var valueStr string
		switch v := value.(type) {
		case string:
			valueStr = v
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		conditions = append(conditions, fmt.Sprintf("metadata ->> '%s' = $%d", key, argIndex))
		args = append(args, valueStr)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")
	return whereClause, args
}

// scanSearchResults scans database rows into SearchResult slice
func (p *PgVector) scanSearchResults(rows *sql.Rows) ([]*vectordb.SearchResult, error) {
	var results []*vectordb.SearchResult

	for rows.Next() {
		var doc document.Document
		var embeddingsVector pgvector.Vector
		var distance float64
		var metadataJSON sql.NullString

		err := rows.Scan(
			&doc.ID,
			&doc.Name,
			&doc.Content,
			&doc.ContentType,
			&metadataJSON,
			&doc.Source,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&embeddingsVector,
			&doc.ChunkIndex,
			&doc.ChunkTotal,
			&doc.ParentID,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse metadata JSON
		if metadataJSON.Valid && metadataJSON.String != "" {
			err = json.Unmarshal([]byte(metadataJSON.String), &doc.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		// Convert embeddings from pgvector.Vector to []float64
		if embeddingsVector.Slice() != nil {
			doc.Embeddings = convertFloat32ToFloat64(embeddingsVector.Slice())
		}

		// Calculate score based on distance metric
		var score float64
		switch p.Distance {
		case vectordb.DistanceCosine:
			// For cosine distance, 0 = identical, 2 = opposite
			// Convert to similarity: 1 - (distance / 2)
			score = 1.0 - (distance / 2.0)
		case vectordb.DistanceL2:
			// For L2 distance, smaller is better
			score = 1.0 / (1.0 + distance)
		case vectordb.DistanceMaxInnerProduct:
			// For inner product, larger (less negative) is better
			// Normalize to 0-1 range by adding offset and scaling
			score = math.Max(0, distance+10) / 10.0
		default:
			score = 1.0 / (1.0 + distance)
		}

		// Ensure score is between 0 and 1
		score = math.Max(0, math.Min(1, score))

		results = append(results, &vectordb.SearchResult{
			Document: &doc,
			Score:    score,
			Distance: distance,
		})
	}

	return results, rows.Err()
}

// Close closes the database connection
func (p *PgVector) Close() error {
	return p.db.Close()
}

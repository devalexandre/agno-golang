// Package singlestore implements the vectordb.VectorDB interface for SingleStore
// (MemSQL) using the MySQL wire protocol via database/sql. Vectors are stored as
// packed blobs and searched with SingleStore's DOT_PRODUCT / EUCLIDEAN_DISTANCE.
package singlestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// SingleStoreDB implements vectordb.VectorDB backed by SingleStore.
type SingleStoreDB struct {
	*vectordb.BaseVectorDB
	db        *sql.DB
	table     string
	dimension int
}

// Options configures a SingleStoreDB.
type Options struct {
	DSN       string // MySQL DSN, e.g. user:pass@tcp(host:3306)/db
	DB        *sql.DB
	Table     string
	Dimension int
	Embedder  embedder.Embedder
}

// New creates a SingleStoreDB. Provide either DSN or an existing *sql.DB.
func New(opts Options) (*SingleStoreDB, error) {
	if opts.Table == "" {
		opts.Table = "agno_documents"
	}
	dim := opts.Dimension
	if dim <= 0 && opts.Embedder != nil {
		dim = opts.Embedder.GetDimensions()
	}
	db := opts.DB
	if db == nil {
		if opts.DSN == "" {
			return nil, fmt.Errorf("singlestore: DSN or DB is required")
		}
		var err error
		db, err = sql.Open("mysql", opts.DSN)
		if err != nil {
			return nil, fmt.Errorf("singlestore: open: %w", err)
		}
	}
	return &SingleStoreDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceDot),
		db:           db,
		table:        opts.Table,
		dimension:    dim,
	}, nil
}

// vecJSON renders an embedding as a JSON array string for JSON_ARRAY_PACK.
func vecJSON(v []float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Create creates the table (id, content, embedding) if absent.
func (s *SingleStoreDB) Create(ctx context.Context) error {
	if s.dimension <= 0 {
		return fmt.Errorf("singlestore: dimension is required")
	}
	ddl := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(255) PRIMARY KEY,
			content LONGTEXT,
			name VARCHAR(255),
			source VARCHAR(512),
			embedding BLOB
		)`, s.table)
	_, err := s.db.ExecContext(ctx, ddl)
	return err
}

// Exists reports whether the table exists.
func (s *SingleStoreDB) Exists(ctx context.Context) (bool, error) {
	var name string
	err := s.db.QueryRowContext(ctx,
		"SELECT table_name FROM information_schema.tables WHERE table_name = ? LIMIT 1", s.table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Drop removes the table.
func (s *SingleStoreDB) Drop(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+s.table)
	return err
}

// Optimize runs OPTIMIZE TABLE.
func (s *SingleStoreDB) Optimize(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "OPTIMIZE TABLE "+s.table)
	return err
}

// Insert embeds and inserts documents.
func (s *SingleStoreDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := s.EmbedDocuments(docs); err != nil {
		return err
	}
	q := fmt.Sprintf(
		"INSERT INTO %s (id, content, name, source, embedding) VALUES (?, ?, ?, ?, JSON_ARRAY_PACK(?))", s.table)
	for _, d := range docs {
		if _, err := s.db.ExecContext(ctx, q, d.ID, d.Content, d.Name, d.Source, vecJSON(d.Embeddings)); err != nil {
			return err
		}
	}
	return nil
}

// Upsert embeds and upserts documents (ON DUPLICATE KEY UPDATE).
func (s *SingleStoreDB) Upsert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := s.EmbedDocuments(docs); err != nil {
		return err
	}
	q := fmt.Sprintf(
		`INSERT INTO %s (id, content, name, source, embedding) VALUES (?, ?, ?, ?, JSON_ARRAY_PACK(?))
		 ON DUPLICATE KEY UPDATE content=VALUES(content), name=VALUES(name), source=VALUES(source), embedding=VALUES(embedding)`,
		s.table)
	for _, d := range docs {
		if _, err := s.db.ExecContext(ctx, q, d.ID, d.Content, d.Name, d.Source, vecJSON(d.Embeddings)); err != nil {
			return err
		}
	}
	return nil
}

// Search performs a vector search.
func (s *SingleStoreDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return s.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch ranks by DOT_PRODUCT similarity (higher is more similar).
func (s *SingleStoreDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := s.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf(
		`SELECT id, content, name, source, DOT_PRODUCT(embedding, JSON_ARRAY_PACK(?)) AS score
		 FROM %s ORDER BY score DESC LIMIT ?`, s.table)
	rows, err := s.db.QueryContext(ctx, q, vecJSON(vec), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*vectordb.SearchResult
	for rows.Next() {
		var id, content, name, source string
		var score float64
		if err := rows.Scan(&id, &content, &name, &source, &score); err != nil {
			return nil, err
		}
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: id, Content: content, Name: name, Source: source},
			Score:    score,
		})
	}
	return results, rows.Err()
}

// KeywordSearch uses a LIKE match as a basic fallback.
func (s *SingleStoreDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	q := fmt.Sprintf("SELECT id, content, name, source FROM %s WHERE content LIKE ? LIMIT ?", s.table)
	rows, err := s.db.QueryContext(ctx, q, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*vectordb.SearchResult
	for rows.Next() {
		var id, content, name, source string
		if err := rows.Scan(&id, &content, &name, &source); err != nil {
			return nil, err
		}
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: id, Content: content, Name: name, Source: source},
		})
	}
	return results, rows.Err()
}

// HybridSearch is not implemented.
func (s *SingleStoreDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("singlestore: hybrid search not implemented")
}

// GetCount returns the number of rows.
func (s *SingleStoreDB) GetCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+s.table).Scan(&n)
	return n, err
}

// DocExists checks existence by the document's ID.
func (s *SingleStoreDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return s.IDExists(ctx, doc.ID)
}

// NameExists checks whether a row with the given name exists.
func (s *SingleStoreDB) NameExists(ctx context.Context, name string) (bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE name = ? LIMIT 1", s.table), name).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// IDExists checks whether a row with the given id exists.
func (s *SingleStoreDB) IDExists(ctx context.Context, id string) (bool, error) {
	var got string
	err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT id FROM %s WHERE id = ? LIMIT 1", s.table), id).Scan(&got)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// Close closes the underlying database connection.
func (s *SingleStoreDB) Close() error { return s.db.Close() }

var _ vectordb.VectorDB = (*SingleStoreDB)(nil)

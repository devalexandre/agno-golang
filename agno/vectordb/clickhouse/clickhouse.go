// Package clickhouse implements the vectordb.VectorDB interface for ClickHouse
// using github.com/ClickHouse/clickhouse-go/v2. Embeddings are stored as
// Array(Float32) and searched with cosineDistance.
package clickhouse

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// ClickHouseDB implements vectordb.VectorDB backed by ClickHouse.
type ClickHouseDB struct {
	*vectordb.BaseVectorDB
	conn  driver.Conn
	table string
}

// Options configures a ClickHouseDB.
type Options struct {
	Addr     []string // e.g. []string{"localhost:9000"}
	Database string
	Username string
	Password string
	Conn     driver.Conn // optional pre-built connection
	Table    string
	Embedder embedder.Embedder
}

// New creates a ClickHouseDB. Provide either Addr or an existing Conn.
func New(opts Options) (*ClickHouseDB, error) {
	if opts.Table == "" {
		opts.Table = "agno_documents"
	}
	conn := opts.Conn
	if conn == nil {
		if len(opts.Addr) == 0 {
			return nil, fmt.Errorf("clickhouse: Addr or Conn is required")
		}
		db := opts.Database
		if db == "" {
			db = "default"
		}
		username := opts.Username
		if username == "" {
			username = "default"
		}
		var err error
		conn, err = ch.Open(&ch.Options{
			Addr: opts.Addr,
			Auth: ch.Auth{Database: db, Username: username, Password: opts.Password},
		})
		if err != nil {
			return nil, fmt.Errorf("clickhouse: open: %w", err)
		}
	}
	return &ClickHouseDB{
		BaseVectorDB: vectordb.NewBaseVectorDB(opts.Embedder, vectordb.SearchTypeVector, vectordb.DistanceCosine),
		conn:         conn,
		table:        opts.Table,
	}, nil
}

// toFloat32 converts a float64 vector to float32 (ClickHouse Array(Float32)).
func toFloat32(v []float64) []float32 {
	out := make([]float32, len(v))
	for i, f := range v {
		out[i] = float32(f)
	}
	return out
}

// Create creates the table if it does not exist.
func (c *ClickHouseDB) Create(ctx context.Context) error {
	ddl := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (
			id String,
			content String,
			name String,
			source String,
			embedding Array(Float32)
		) ENGINE = MergeTree() ORDER BY id`, c.table)
	return c.conn.Exec(ctx, ddl)
}

// Exists reports whether the table exists.
func (c *ClickHouseDB) Exists(ctx context.Context) (bool, error) {
	var n uint64
	err := c.conn.QueryRow(ctx, "SELECT count() FROM system.tables WHERE name = ?", c.table).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Drop removes the table.
func (c *ClickHouseDB) Drop(ctx context.Context) error {
	return c.conn.Exec(ctx, "DROP TABLE IF EXISTS "+c.table)
}

// Optimize runs OPTIMIZE TABLE FINAL.
func (c *ClickHouseDB) Optimize(ctx context.Context) error {
	return c.conn.Exec(ctx, "OPTIMIZE TABLE "+c.table+" FINAL")
}

// Insert embeds and inserts documents via a batch.
func (c *ClickHouseDB) Insert(ctx context.Context, docs []*document.Document, _ map[string]interface{}) error {
	if err := c.EmbedDocuments(docs); err != nil {
		return err
	}
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO "+c.table+" (id, content, name, source, embedding)")
	if err != nil {
		return err
	}
	for _, d := range docs {
		if err := batch.Append(d.ID, d.Content, d.Name, d.Source, toFloat32(d.Embeddings)); err != nil {
			return err
		}
	}
	return batch.Send()
}

// Upsert inserts documents (MergeTree dedups on OPTIMIZE; for true upsert use ReplacingMergeTree).
func (c *ClickHouseDB) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return c.Insert(ctx, docs, filters)
}

// Search performs a vector search.
func (c *ClickHouseDB) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return c.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch ranks by cosineDistance (ascending = most similar).
func (c *ClickHouseDB) VectorSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	vec, err := c.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf(
		`SELECT id, content, name, source, cosineDistance(embedding, ?) AS dist
		 FROM %s ORDER BY dist ASC LIMIT ?`, c.table)
	rows, err := c.conn.Query(ctx, q, toFloat32(vec), uint64(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*vectordb.SearchResult
	for rows.Next() {
		var id, content, name, source string
		var dist float64
		if err := rows.Scan(&id, &content, &name, &source, &dist); err != nil {
			return nil, err
		}
		results = append(results, &vectordb.SearchResult{
			Document: &document.Document{ID: id, Content: content, Name: name, Source: source},
			Score:    1 - dist,
			Distance: dist,
		})
	}
	return results, rows.Err()
}

// KeywordSearch runs a substring match on content.
func (c *ClickHouseDB) KeywordSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	q := fmt.Sprintf("SELECT id, content, name, source FROM %s WHERE position(content, ?) > 0 LIMIT ?", c.table)
	rows, err := c.conn.Query(ctx, q, query, uint64(limit))
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
func (c *ClickHouseDB) HybridSearch(ctx context.Context, query string, limit int, _ map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, fmt.Errorf("clickhouse: hybrid search not implemented")
}

// GetCount returns the number of rows.
func (c *ClickHouseDB) GetCount(ctx context.Context) (int64, error) {
	var n uint64
	err := c.conn.QueryRow(ctx, "SELECT count() FROM "+c.table).Scan(&n)
	return int64(n), err
}

// DocExists checks existence by the document's ID.
func (c *ClickHouseDB) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	return c.IDExists(ctx, doc.ID)
}

// NameExists checks whether a row with the given name exists.
func (c *ClickHouseDB) NameExists(ctx context.Context, name string) (bool, error) {
	var n uint64
	err := c.conn.QueryRow(ctx, "SELECT count() FROM "+c.table+" WHERE name = ?", name).Scan(&n)
	return n > 0, err
}

// IDExists checks whether a row with the given id exists.
func (c *ClickHouseDB) IDExists(ctx context.Context, id string) (bool, error) {
	var n uint64
	err := c.conn.QueryRow(ctx, "SELECT count() FROM "+c.table+" WHERE id = ?", id).Scan(&n)
	return n > 0, err
}

// Close closes the ClickHouse connection.
func (c *ClickHouseDB) Close() error { return c.conn.Close() }

var _ vectordb.VectorDB = (*ClickHouseDB)(nil)

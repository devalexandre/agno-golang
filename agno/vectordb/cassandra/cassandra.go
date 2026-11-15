// Package cassandra implements the vectordb.VectorDB interface on Apache
// Cassandra 5 (or DataStax Astra / HCD) native vector search, mirroring Agno
// Python's agno.vectordb.cassandra (which uses cassio's metadata-vector table).
//
// Rows live in a table with the cassio layout:
//
//	row_id text PRIMARY KEY, body_blob text, vector vector<float, N>,
//	metadata_s map<text, text>, document_name text
//
// and an SAI index on the vector column. Searches use
// "ORDER BY vector ANN OF ?" with the similarity function matching the
// configured Distance. Like Python, only vector search is supported; keyword
// and hybrid search return ErrUnsupportedSearch.
//
// The package talks to Cassandra through the small Session interface so it can
// be tested without a cluster; NewWithGocql builds one on top of
// github.com/apache/cassandra-gocql-driver/v2 (which supports vector<float,N>).
package cassandra

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/embedder"
	"github.com/devalexandre/agno-golang/agno/vectordb"
)

// ErrUnsupportedSearch is returned by KeywordSearch and HybridSearch: Cassandra
// (like Agno Python's implementation) only supports vector search.
var ErrUnsupportedSearch = errors.New("cassandra: only vector search is supported")

// Rows iterates the result of a query. Scan fills one row into a map keyed by
// column name and reports whether a row was read; Close returns the query error.
type Rows interface {
	MapScan(m map[string]interface{}) bool
	Close() error
}

// Session is the minimal CQL surface the vector db needs.
type Session interface {
	Exec(ctx context.Context, stmt string, args ...interface{}) error
	Query(ctx context.Context, stmt string, args ...interface{}) Rows
}

// Config configures a Cassandra vector DB.
type Config struct {
	Keyspace  string
	TableName string
	Embedder  embedder.Embedder
	// Dimensions of the vector column; defaults to the embedder's.
	Dimensions int
	// Distance selects the SAI similarity function: cosine (default),
	// dot/max_inner_product -> dot_product, l2/euclidean -> euclidean.
	Distance vectordb.Distance
}

// Cassandra implements vectordb.VectorDB over CQL.
type Cassandra struct {
	*vectordb.BaseVectorDB
	session  Session
	keyspace string
	table    string
	simFn    string
}

var _ vectordb.VectorDB = (*Cassandra)(nil)

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// New creates a Cassandra vector DB over an existing Session.
func New(session Session, cfg Config) (*Cassandra, error) {
	if session == nil {
		return nil, errors.New("cassandra: session is required")
	}
	if cfg.Keyspace == "" || cfg.TableName == "" {
		return nil, errors.New("cassandra: keyspace and table name are required")
	}
	if !identRe.MatchString(cfg.Keyspace) || !identRe.MatchString(cfg.TableName) {
		return nil, fmt.Errorf("cassandra: invalid keyspace/table identifier %q.%q", cfg.Keyspace, cfg.TableName)
	}
	if cfg.Embedder == nil {
		return nil, errors.New("cassandra: embedder is required")
	}
	if cfg.Distance == "" {
		cfg.Distance = vectordb.DistanceCosine
	}
	base := vectordb.NewBaseVectorDB(cfg.Embedder, vectordb.SearchTypeVector, cfg.Distance)
	if cfg.Dimensions > 0 {
		base.Dimensions = cfg.Dimensions
	}
	return &Cassandra{
		BaseVectorDB: base,
		session:      session,
		keyspace:     cfg.Keyspace,
		table:        cfg.TableName,
		simFn:        similarityFunction(cfg.Distance),
	}, nil
}

func similarityFunction(d vectordb.Distance) string {
	switch d {
	case vectordb.DistanceDot, vectordb.DistanceIP, vectordb.DistanceMaxInnerProduct:
		return "dot_product"
	case vectordb.DistanceL2, vectordb.DistanceEuclidean:
		return "euclidean"
	default:
		return "cosine"
	}
}

func (c *Cassandra) fqtn() string { return c.keyspace + "." + c.table }

// Create creates the table and the SAI vector index if they do not exist.
func (c *Cassandra) Create(ctx context.Context) error {
	if c.Dimensions <= 0 {
		return errors.New("cassandra: dimensions are required to create the table")
	}
	stmts := []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (row_id text PRIMARY KEY, body_blob text, vector vector<float, %d>, metadata_s map<text, text>, document_name text)", c.fqtn(), c.Dimensions),
		fmt.Sprintf("CREATE CUSTOM INDEX IF NOT EXISTS %s_vector_idx ON %s (vector) USING 'StorageAttachedIndex' WITH OPTIONS = {'similarity_function': '%s'}", c.table, c.fqtn(), c.simFn),
		fmt.Sprintf("CREATE CUSTOM INDEX IF NOT EXISTS %s_metadata_idx ON %s (ENTRIES(metadata_s)) USING 'StorageAttachedIndex'", c.table, c.fqtn()),
	}
	for _, s := range stmts {
		if err := c.session.Exec(ctx, s); err != nil {
			return fmt.Errorf("cassandra: create: %w", err)
		}
	}
	return nil
}

// Exists reports whether the table exists (system_schema lookup).
func (c *Cassandra) Exists(ctx context.Context) (bool, error) {
	rows := c.session.Query(ctx, "SELECT table_name FROM system_schema.tables WHERE keyspace_name = ? AND table_name = ?", c.keyspace, c.table)
	found := rows.MapScan(map[string]interface{}{})
	if err := rows.Close(); err != nil {
		return false, err
	}
	return found, nil
}

// Drop drops the table.
func (c *Cassandra) Drop(ctx context.Context) error {
	return c.session.Exec(ctx, "DROP TABLE IF EXISTS "+c.fqtn())
}

// Optimize is a no-op (SAI indexes are maintained automatically).
func (c *Cassandra) Optimize(ctx context.Context) error { return nil }

// rowID derives a stable row id: the document ID when present, else the md5
// of its content (Python uses md5(f"{base_id}_{content_hash}")).
func rowID(doc *document.Document) string {
	if doc.ID != "" {
		return doc.ID
	}
	sum := md5.Sum([]byte(doc.Content))
	return hex.EncodeToString(sum[:])
}

func toFloat32(v []float64) []float32 {
	out := make([]float32, len(v))
	for i, f := range v {
		out[i] = float32(f)
	}
	return out
}

func metadataMap(doc *document.Document, filters map[string]interface{}) map[string]string {
	m := make(map[string]string, len(doc.Metadata)+len(filters)+2)
	for k, v := range doc.Metadata {
		m[k] = fmt.Sprint(v)
	}
	for k, v := range filters {
		m[k] = fmt.Sprint(v)
	}
	if doc.Source != "" {
		m["source"] = doc.Source
	}
	return m
}

// Insert embeds and writes documents. CQL INSERT is an upsert.
func (c *Cassandra) Insert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	if err := c.EmbedDocuments(docs); err != nil {
		return err
	}
	stmt := fmt.Sprintf("INSERT INTO %s (row_id, body_blob, vector, metadata_s, document_name) VALUES (?, ?, ?, ?, ?)", c.fqtn())
	for _, d := range docs {
		if d == nil {
			continue
		}
		if err := c.session.Exec(ctx, stmt, rowID(d), d.Content, toFloat32(d.Embeddings), metadataMap(d, filters), d.Name); err != nil {
			return fmt.Errorf("cassandra: insert %s: %w", rowID(d), err)
		}
	}
	return nil
}

// Upsert is equivalent to Insert.
func (c *Cassandra) Upsert(ctx context.Context, docs []*document.Document, filters map[string]interface{}) error {
	return c.Insert(ctx, docs, filters)
}

// Search runs a vector search.
func (c *Cassandra) Search(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return c.VectorSearch(ctx, query, limit, filters)
}

// VectorSearch runs an ANN query. Filters are applied as equality on
// metadata_s entries (served by the ENTRIES SAI index).
func (c *Cassandra) VectorSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	vec, err := c.EmbedQuery(query)
	if err != nil {
		return nil, err
	}
	if len(vec) == 0 {
		return nil, errors.New("cassandra: empty query embedding")
	}
	qv := toFloat32(vec)
	args := []interface{}{qv}
	var where []string
	for _, k := range sortedKeys(filters) {
		where = append(where, "metadata_s['"+strings.ReplaceAll(k, "'", "''")+"'] = ?")
		args = append(args, fmt.Sprint(filters[k]))
	}
	stmt := fmt.Sprintf("SELECT row_id, body_blob, metadata_s, document_name, similarity_%s(vector, ?) AS score FROM %s", c.simFn, c.fqtn())
	if len(where) > 0 {
		stmt += " WHERE " + strings.Join(where, " AND ")
	}
	stmt += " ORDER BY vector ANN OF ? LIMIT ?"
	args = append(args, qv, limit)

	rows := c.session.Query(ctx, stmt, args...)
	var out []*vectordb.SearchResult
	for {
		row := map[string]interface{}{}
		if !rows.MapScan(row) {
			break
		}
		out = append(out, rowToResult(row))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("cassandra: search: %w", err)
	}
	return out, nil
}

func rowToResult(row map[string]interface{}) *vectordb.SearchResult {
	doc := &document.Document{Metadata: map[string]interface{}{}}
	doc.ID, _ = row["row_id"].(string)
	doc.Content, _ = row["body_blob"].(string)
	doc.Name, _ = row["document_name"].(string)
	if md, ok := row["metadata_s"].(map[string]string); ok {
		for k, v := range md {
			if k == "source" {
				doc.Source = v
			}
			doc.Metadata[k] = v
		}
	}
	var score float64
	switch s := row["score"].(type) {
	case float32:
		score = float64(s)
	case float64:
		score = s
	}
	return &vectordb.SearchResult{Document: doc, Score: score, Distance: 1 - score}
}

// KeywordSearch is not supported.
func (c *Cassandra) KeywordSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, ErrUnsupportedSearch
}

// HybridSearch is not supported.
func (c *Cassandra) HybridSearch(ctx context.Context, query string, limit int, filters map[string]interface{}) ([]*vectordb.SearchResult, error) {
	return nil, ErrUnsupportedSearch
}

func (c *Cassandra) count(ctx context.Context, stmt string, args ...interface{}) (int64, error) {
	rows := c.session.Query(ctx, stmt, args...)
	row := map[string]interface{}{}
	ok := rows.MapScan(row)
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	switch n := row["count"].(type) {
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	case int32:
		return int64(n), nil
	}
	return 0, nil
}

// GetCount returns the number of rows (a full scan, as in Python).
func (c *Cassandra) GetCount(ctx context.Context) (int64, error) {
	return c.count(ctx, "SELECT COUNT(*) FROM "+c.fqtn())
}

// DocExists checks existence by the document's row id.
func (c *Cassandra) DocExists(ctx context.Context, doc *document.Document) (bool, error) {
	if doc == nil {
		return false, nil
	}
	return c.IDExists(ctx, rowID(doc))
}

// NameExists checks whether a row has the given document_name.
func (c *Cassandra) NameExists(ctx context.Context, name string) (bool, error) {
	n, err := c.count(ctx, "SELECT COUNT(*) FROM "+c.fqtn()+" WHERE document_name = ? ALLOW FILTERING", name)
	return n > 0, err
}

// IDExists checks whether a row with the given id exists.
func (c *Cassandra) IDExists(ctx context.Context, id string) (bool, error) {
	n, err := c.count(ctx, "SELECT COUNT(*) FROM "+c.fqtn()+" WHERE row_id = ?", id)
	return n > 0, err
}

// DeleteByID deletes one row.
func (c *Cassandra) DeleteByID(ctx context.Context, id string) error {
	return c.session.Exec(ctx, "DELETE FROM "+c.fqtn()+" WHERE row_id = ?", id)
}

// DeleteByName deletes every row with the given document_name.
func (c *Cassandra) DeleteByName(ctx context.Context, name string) error {
	rows := c.session.Query(ctx, "SELECT row_id FROM "+c.fqtn()+" WHERE document_name = ? ALLOW FILTERING", name)
	var ids []string
	for {
		row := map[string]interface{}{}
		if !rows.MapScan(row) {
			break
		}
		if id, ok := row["row_id"].(string); ok {
			ids = append(ids, id)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.DeleteByID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

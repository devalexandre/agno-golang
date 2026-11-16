// Package clickhouse provides a ClickHouse-backed implementation of the
// storage contracts (storage.DB, storage.Storage, storage.KnowledgeStorage,
// session search and metrics), a port of Python agno/db/clickhouse.
//
// ClickHouse is append-oriented, so documents live in one
// ReplacingMergeTree table "{prefix}_documents" keyed by (collection, id):
// every write inserts a new version, deletes insert a tombstone, and reads
// pick the latest version with argMax — correct before and after merges.
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2" // registers the "clickhouse" driver

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
)

// Config configures the ClickHouse backend.
type Config struct {
	// DSN is a clickhouse-go DSN, e.g. clickhouse://default:@localhost:9000/default.
	// Defaults to CLICKHOUSE_DSN.
	DSN string
	// DB is an existing connection; when set DSN is ignored.
	DB *sql.DB
	// Prefix is the table name prefix (default "agno").
	Prefix string
}

// Backend is a kvstore.Backend over ClickHouse.
type Backend struct {
	db    *sql.DB
	table string

	mu   sync.Mutex
	last uint64
}

// NewBackend opens the connection and creates a backend.
func NewBackend(cfg Config) (*Backend, error) {
	db := cfg.DB
	if db == nil {
		dsn := cfg.DSN
		if dsn == "" {
			dsn = os.Getenv("CLICKHOUSE_DSN")
		}
		if dsn == "" {
			return nil, fmt.Errorf("clickhouse: DSN is required (set CLICKHOUSE_DSN)")
		}
		var err error
		db, err = sql.Open("clickhouse", dsn)
		if err != nil {
			return nil, fmt.Errorf("clickhouse: open: %w", err)
		}
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agno"
	}
	return &Backend{db: db, table: prefix + "_documents"}, nil
}

// New creates a ClickHouse-backed store implementing every storage contract.
func New(cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("clickhouse")}, opts...)...), nil
}

// DB returns the underlying connection.
func (b *Backend) DB() *sql.DB { return b.db }

// version returns a strictly increasing version number.
func (b *Backend) version() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	v := uint64(time.Now().UnixNano())
	if v <= b.last {
		v = b.last + 1
	}
	b.last = v
	return v
}

// CreateTableSQL returns the DDL of the documents table.
func (b *Backend) CreateTableSQL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	collection String,
	id String,
	data String,
	idx Map(String, String),
	version UInt64,
	is_deleted UInt8
) ENGINE = ReplacingMergeTree(version)
ORDER BY (collection, id)`, b.table)
}

// Init creates the documents table.
func (b *Backend) Init(ctx context.Context) error {
	_, err := b.db.ExecContext(ctx, b.CreateTableSQL())
	return err
}

func (b *Backend) insert(ctx context.Context, coll, id string, doc []byte, index map[string]string, deleted uint8) error {
	if index == nil {
		index = map[string]string{}
	}
	q := fmt.Sprintf(`INSERT INTO %s (collection, id, data, idx, version, is_deleted) VALUES (?, ?, ?, ?, ?, ?)`, b.table)
	_, err := b.db.ExecContext(ctx, q, coll, id, string(doc), index, b.version(), deleted)
	return err
}

// Put writes a new version of a document.
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, index map[string]string) error {
	return b.insert(ctx, coll, id, doc, index, 0)
}

// Get returns the latest live version of a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	q := fmt.Sprintf(`SELECT argMax(data, version), argMax(is_deleted, version) FROM %s
WHERE collection = ? AND id = ? GROUP BY id`, b.table)
	var (
		data    string
		deleted uint8
	)
	err := b.db.QueryRowContext(ctx, q, coll, id).Scan(&data, &deleted)
	if err == sql.ErrNoRows || (err == nil && deleted == 1) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []byte(data), nil
}

// Delete writes a tombstone.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	return b.insert(ctx, coll, id, nil, nil, 1)
}

// ListQuery builds the List query and its args.
func (b *Backend) ListQuery(coll string, where map[string]string) (string, []interface{}) {
	q := fmt.Sprintf(`SELECT id, argMax(data, version) AS d FROM %s WHERE collection = ? GROUP BY id
HAVING argMax(is_deleted, version) = 0`, b.table)
	args := []interface{}{coll}
	for k, v := range where {
		q += ` AND argMax(idx, version)[?] = ?`
		args = append(args, k, v)
	}
	return q + ` ORDER BY id`, args
}

// List returns the latest live documents matching where.
func (b *Backend) List(ctx context.Context, coll string, where map[string]string) ([][]byte, error) {
	q, args := b.ListQuery(coll, where)
	rows, err := b.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		out = append(out, []byte(data))
	}
	return out, rows.Err()
}

// Drop deletes every version of a collection (lightweight DELETE, ClickHouse 23.3+).
func (b *Backend) Drop(ctx context.Context, coll string) error {
	_, err := b.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE collection = ?`, b.table), coll)
	return err
}

var _ kvstore.Backend = (*Backend)(nil)

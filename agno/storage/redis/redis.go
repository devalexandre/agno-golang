// Package redis provides a Redis-backed implementation of the storage
// contracts (storage.DB, storage.Storage, storage.KnowledgeStorage, session
// search and metrics), a port of Python agno/db/redis.
//
// Layout (same key scheme as Python): documents are JSON strings at
// "{prefix}:{collection}:{id}", every id is in the set
// "{prefix}:{collection}:ids", and indexed fields are kept in sets
// "{prefix}:{collection}:index:{field}:{value}". The package also works with
// Valkey, KeyDB and other Redis-protocol servers.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
)

// Config configures the Redis backend.
type Config struct {
	// URL is a redis:// URL. Defaults to REDIS_URL, then redis://localhost:6379/0.
	URL string
	// Client is an existing client; when set URL is ignored.
	Client goredis.UniversalClient
	// Prefix is the key prefix (default "agno").
	Prefix string
	// Expire sets a TTL on every document when > 0.
	Expire time.Duration
}

// Backend is a kvstore.Backend over Redis.
type Backend struct {
	client goredis.UniversalClient
	prefix string
	expire time.Duration
}

// NewBackend creates a Redis backend.
func NewBackend(cfg Config) (*Backend, error) {
	client := cfg.Client
	if client == nil {
		url := cfg.URL
		if url == "" {
			url = os.Getenv("REDIS_URL")
		}
		if url == "" {
			url = "redis://localhost:6379/0"
		}
		opts, err := goredis.ParseURL(url)
		if err != nil {
			return nil, fmt.Errorf("redis: parse url: %w", err)
		}
		client = goredis.NewClient(opts)
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "agno"
	}
	return &Backend{client: client, prefix: prefix, expire: cfg.Expire}, nil
}

// New creates a Redis-backed store implementing every storage contract.
func New(cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("redis")}, opts...)...), nil
}

// Client returns the underlying client.
func (b *Backend) Client() goredis.UniversalClient { return b.client }

func (b *Backend) docKey(coll, id string) string { return fmt.Sprintf("%s:%s:%s", b.prefix, coll, id) }
func (b *Backend) idsKey(coll string) string     { return fmt.Sprintf("%s:%s:ids", b.prefix, coll) }
func (b *Backend) metaKey(coll, id string) string {
	return fmt.Sprintf("%s:%s:meta:%s", b.prefix, coll, id)
}
func (b *Backend) indexKey(coll, field, value string) string {
	return fmt.Sprintf("%s:%s:index:%s:%s", b.prefix, coll, field, value)
}

// Init pings the server.
func (b *Backend) Init(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}

func (b *Backend) writeIndex(ctx context.Context, pipe goredis.Pipeliner, coll, id string, old, index map[string]string) {
	for f, v := range old {
		pipe.SRem(ctx, b.indexKey(coll, f, v), id)
	}
	pipe.Del(ctx, b.metaKey(coll, id))
	if len(index) > 0 {
		raw, _ := json.Marshal(index)
		pipe.Set(ctx, b.metaKey(coll, id), raw, b.expire)
		for f, v := range index {
			pipe.SAdd(ctx, b.indexKey(coll, f, v), id)
		}
	}
	pipe.SAdd(ctx, b.idsKey(coll), id)
}

func (b *Backend) oldIndex(ctx context.Context, coll, id string) (map[string]string, error) {
	raw, err := b.client.Get(ctx, b.metaKey(coll, id)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	return m, nil
}

// Put writes a document and its index entries.
func (b *Backend) Put(ctx context.Context, coll, id string, doc []byte, index map[string]string) error {
	old, err := b.oldIndex(ctx, coll, id)
	if err != nil {
		return err
	}
	_, err = b.client.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.Set(ctx, b.docKey(coll, id), doc, b.expire)
		b.writeIndex(ctx, pipe, coll, id, old, index)
		return nil
	})
	return err
}

// Insert writes a document only if absent (SET NX).
func (b *Backend) Insert(ctx context.Context, coll, id string, doc []byte, index map[string]string) (bool, error) {
	ok, err := b.client.SetNX(ctx, b.docKey(coll, id), doc, b.expire).Result()
	if err != nil || !ok {
		return false, err
	}
	_, err = b.client.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		b.writeIndex(ctx, pipe, coll, id, nil, index)
		return nil
	})
	return true, err
}

// Get reads a document.
func (b *Backend) Get(ctx context.Context, coll, id string) ([]byte, error) {
	raw, err := b.client.Get(ctx, b.docKey(coll, id)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	return raw, err
}

// Delete removes a document and its index entries.
func (b *Backend) Delete(ctx context.Context, coll, id string) error {
	old, err := b.oldIndex(ctx, coll, id)
	if err != nil {
		return err
	}
	_, err = b.client.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.Del(ctx, b.docKey(coll, id), b.metaKey(coll, id))
		pipe.SRem(ctx, b.idsKey(coll), id)
		for f, v := range old {
			pipe.SRem(ctx, b.indexKey(coll, f, v), id)
		}
		return nil
	})
	return err
}

// List returns the documents matching where (index-set intersection).
func (b *Backend) List(ctx context.Context, coll string, where map[string]string) ([][]byte, error) {
	var ids []string
	var err error
	if len(where) == 0 {
		ids, err = b.client.SMembers(ctx, b.idsKey(coll)).Result()
	} else {
		keys := make([]string, 0, len(where))
		for f, v := range where {
			keys = append(keys, b.indexKey(coll, f, v))
		}
		ids, err = b.client.SInter(ctx, keys...).Result()
	}
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = b.docKey(coll, id)
	}
	vals, err := b.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(vals))
	for _, v := range vals {
		if s, ok := v.(string); ok { // nil when expired
			out = append(out, []byte(s))
		}
	}
	return out, nil
}

// Drop deletes every key of a collection.
func (b *Backend) Drop(ctx context.Context, coll string) error {
	iter := b.client.Scan(ctx, 0, fmt.Sprintf("%s:%s:*", b.prefix, coll), 500).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) >= 500 {
			if err := b.client.Del(ctx, batch...).Err(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(batch) > 0 {
		return b.client.Del(ctx, batch...).Err()
	}
	return nil
}

var (
	_ kvstore.Backend  = (*Backend)(nil)
	_ kvstore.Inserter = (*Backend)(nil)
)

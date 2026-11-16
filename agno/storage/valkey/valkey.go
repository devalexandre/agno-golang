// Package valkey provides a Valkey-backed implementation of the storage
// contracts, a port of Python agno/db/valkey. Valkey speaks the Redis
// protocol, so this package reuses the redis backend (same key layout).
package valkey

import (
	"os"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/devalexandre/agno-golang/agno/storage/kvstore"
	agnoredis "github.com/devalexandre/agno-golang/agno/storage/redis"
)

// Config configures the Valkey backend.
type Config struct {
	// URL is a redis:// or valkey:// URL. Defaults to VALKEY_URL, then redis://localhost:6379/0.
	URL string
	// Client is an existing client; when set URL is ignored.
	Client goredis.UniversalClient
	// Prefix is the key prefix (default "agno").
	Prefix string
	// Expire sets a TTL on every document when > 0.
	Expire time.Duration
}

func (c Config) redis() agnoredis.Config {
	url := c.URL
	if url == "" {
		url = os.Getenv("VALKEY_URL")
	}
	if len(url) > 9 && url[:9] == "valkey://" {
		url = "redis://" + url[9:]
	} else if len(url) > 10 && url[:10] == "valkeys://" {
		url = "rediss://" + url[10:]
	}
	if url == "" && c.Client == nil {
		url = "redis://localhost:6379/0"
	}
	return agnoredis.Config{URL: url, Client: c.Client, Prefix: c.Prefix, Expire: c.Expire}
}

// NewBackend creates a Valkey kvstore backend.
func NewBackend(cfg Config) (*agnoredis.Backend, error) { return agnoredis.NewBackend(cfg.redis()) }

// New creates a Valkey-backed store implementing every storage contract.
func New(cfg Config, opts ...kvstore.Option) (*kvstore.Store, error) {
	b, err := NewBackend(cfg)
	if err != nil {
		return nil, err
	}
	return kvstore.New(b, append([]kvstore.Option{kvstore.WithID("valkey")}, opts...)...), nil
}

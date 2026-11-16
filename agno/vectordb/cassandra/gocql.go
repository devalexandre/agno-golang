package cassandra

import (
	"context"
	"fmt"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

// GocqlConfig configures a cluster connection for NewWithGocql.
type GocqlConfig struct {
	Hosts       []string // defaults to 127.0.0.1
	Username    string
	Password    string
	Consistency gocql.Consistency // defaults to LocalQuorum
	Timeout     time.Duration     // defaults to 10s
}

// gocqlSession adapts *gocql.Session to Session.
type gocqlSession struct{ s *gocql.Session }

// NewGocqlSession wraps an existing driver session.
func NewGocqlSession(s *gocql.Session) Session { return &gocqlSession{s: s} }

func (g *gocqlSession) Exec(ctx context.Context, stmt string, args ...interface{}) error {
	return g.s.Query(stmt, args...).ExecContext(ctx)
}

func (g *gocqlSession) Query(ctx context.Context, stmt string, args ...interface{}) Rows {
	return g.s.Query(stmt, args...).IterContext(ctx)
}

// NewWithGocql connects to the cluster with the Apache Cassandra Go driver
// and returns a vector DB plus a close function for the session.
func NewWithGocql(conn GocqlConfig, cfg Config) (*Cassandra, func(), error) {
	hosts := conn.Hosts
	if len(hosts) == 0 {
		hosts = []string{"127.0.0.1"}
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = cfg.Keyspace
	cluster.Consistency = gocql.LocalQuorum
	if conn.Consistency != 0 {
		cluster.Consistency = conn.Consistency
	}
	cluster.Timeout = 10 * time.Second
	if conn.Timeout > 0 {
		cluster.Timeout = conn.Timeout
	}
	if conn.Username != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{Username: conn.Username, Password: conn.Password}
	}
	s, err := cluster.CreateSession()
	if err != nil {
		return nil, nil, fmt.Errorf("cassandra: connect: %w", err)
	}
	db, err := New(NewGocqlSession(s), cfg)
	if err != nil {
		s.Close()
		return nil, nil, err
	}
	return db, s.Close, nil
}

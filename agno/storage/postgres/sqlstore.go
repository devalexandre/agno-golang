// Package postgres provides a PostgreSQL implementation of storage.DB,
// built on the shared sqlstore core.
package postgres

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/devalexandre/agno-golang/agno/storage/sqlstore"
)

// New opens a PostgreSQL connection and returns a storage.DB.
//
// connStr is a lib/pq connection string, e.g.
//
//	postgres://user:pass@localhost:5432/dbname?sslmode=disable
//
// When connStr is empty the DATABASE_URL env var is used. tablePrefix defaults
// to "agno" (tables: <prefix>_sessions, <prefix>_runs).
func New(connStr, tablePrefix string) (*sqlstore.SQLStore, error) {
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		return nil, fmt.Errorf("postgres: connection string is required (set DATABASE_URL)")
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return NewWithDB(db, tablePrefix), nil
}

// NewWithDB builds a storage.DB from an existing *sql.DB.
func NewWithDB(db *sql.DB, tablePrefix string) *sqlstore.SQLStore {
	return sqlstore.New(db, sqlstore.Dialect{Driver: "postgres", Positional: true}, tablePrefix)
}

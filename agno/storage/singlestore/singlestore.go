// Package singlestore provides a SingleStore implementation of storage.DB
// (plus session search, metrics and knowledge), a port of Python
// agno/db/singlestore. SingleStore speaks the MySQL wire protocol, so the
// backend reuses the shared sqlstore core with the MySQL dialect.
package singlestore

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql" // MySQL-protocol driver

	"github.com/devalexandre/agno-golang/agno/storage/sqlstore"
)

// Dialect is the SingleStore SQL dialect ("?" placeholders, case-insensitive
// default collation).
var Dialect = sqlstore.Dialect{Driver: "mysql", Positional: false, CaseInsensitiveCollation: true}

// New opens a SingleStore connection and returns the store.
//
// dsn is a go-sql-driver/mysql DSN, e.g.
//
//	admin:pass@tcp(svc-xxx.singlestore.com:3306)/agno?tls=true
//
// When dsn is empty SINGLESTORE_DSN is used. tablePrefix defaults to "agno".
func New(dsn, tablePrefix string) (*sqlstore.SQLStore, error) {
	if dsn == "" {
		dsn = os.Getenv("SINGLESTORE_DSN")
	}
	if dsn == "" {
		return nil, fmt.Errorf("singlestore: DSN is required (set SINGLESTORE_DSN)")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("singlestore: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("singlestore: ping: %w", err)
	}
	return NewWithDB(db, tablePrefix), nil
}

// NewWithDB builds the store from an existing *sql.DB.
func NewWithDB(db *sql.DB, tablePrefix string) *sqlstore.SQLStore {
	return sqlstore.New(db, Dialect, tablePrefix)
}

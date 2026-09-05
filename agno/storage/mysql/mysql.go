// Package mysql provides a MySQL implementation of storage.DB, built on the
// shared sqlstore core.
package mysql

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql" // MySQL driver

	"github.com/devalexandre/agno-golang/agno/storage/sqlstore"
)

// New opens a MySQL connection and returns a storage.DB.
//
// dsn is a go-sql-driver/mysql DSN, e.g.
//
//	user:pass@tcp(localhost:3306)/dbname?parseTime=true
//
// When dsn is empty the MYSQL_DSN env var is used. tablePrefix defaults to
// "agno" (tables: <prefix>_sessions, <prefix>_runs).
func New(dsn, tablePrefix string) (*sqlstore.SQLStore, error) {
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		return nil, fmt.Errorf("mysql: DSN is required (set MYSQL_DSN)")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("mysql: ping: %w", err)
	}
	return NewWithDB(db, tablePrefix), nil
}

// NewWithDB builds a storage.DB from an existing *sql.DB.
func NewWithDB(db *sql.DB, tablePrefix string) *sqlstore.SQLStore {
	return sqlstore.New(db, sqlstore.Dialect{Driver: "mysql", Positional: false}, tablePrefix)
}

// Package sqltools ports Agno's SQLTools: list tables, describe a table and
// run SQL against any database/sql connection. Python uses SQLAlchemy; here
// the caller supplies a *sql.DB (import the driver you need, e.g.
// github.com/lib/pq, github.com/go-sql-driver/mysql or modernc.org/sqlite)
// or a driver name + DSN via NewSQLToolsFromDSN.
package sqltools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Dialects understood for catalog queries.
const (
	DialectPostgres = "postgres"
	DialectMySQL    = "mysql"
	DialectSQLite   = "sqlite"
)

// SQLTools exposes a SQL database to an agent.
type SQLTools struct {
	toolkit.Toolkit
	db      *sql.DB
	dialect string
	schema  string
	tables  map[string]interface{}

	enableList, enableDescribe, enableRun bool
}

// Option configures SQLTools.
type Option func(*SQLTools)

// WithDialect sets the dialect (postgres, mysql, sqlite). Auto-detected from the driver when empty.
func WithDialect(d string) Option { return func(s *SQLTools) { s.dialect = d } }

// WithSchema sets the schema used by ListTables/DescribeTable.
func WithSchema(schema string) Option { return func(s *SQLTools) { s.schema = schema } }

// WithTables provides a fixed table description returned by ListTables.
func WithTables(t map[string]interface{}) Option { return func(s *SQLTools) { s.tables = t } }

// WithListTables toggles ListTables (default true).
func WithListTables(b bool) Option { return func(s *SQLTools) { s.enableList = b } }

// WithDescribeTable toggles DescribeTable (default true).
func WithDescribeTable(b bool) Option { return func(s *SQLTools) { s.enableDescribe = b } }

// WithRunSQLQuery toggles RunSQLQuery (default true).
func WithRunSQLQuery(b bool) Option { return func(s *SQLTools) { s.enableRun = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(s *SQLTools) { s.enableList, s.enableDescribe, s.enableRun = true, true, true }
}

// NoParams is used by parameterless methods.
type NoParams struct{}

// DescribeTableParams for DescribeTable.
type DescribeTableParams struct {
	TableName string `json:"table_name" description:"The name of the table to get the schema for." required:"true"`
}

// RunSQLQueryParams for RunSQLQuery.
type RunSQLQueryParams struct {
	Query string `json:"query" description:"The query to run." required:"true"`
	Limit *int   `json:"limit,omitempty" description:"The number of rows to return. Defaults to 10. Use -1 to show all results; 0 returns no rows."`
}

// NewSQLTools wraps an open *sql.DB.
func NewSQLTools(db *sql.DB, opts ...Option) *SQLTools {
	s := &SQLTools{db: db, enableList: true, enableDescribe: true, enableRun: true}
	s.Toolkit = toolkit.NewToolkit()
	s.Toolkit.Name = "sql_tools"
	s.Toolkit.Description = "List tables, describe tables and run SQL queries against a database."
	for _, o := range opts {
		o(s)
	}
	if s.dialect == "" && db != nil {
		s.dialect = DetectDialect(db)
	}
	if s.enableList {
		s.Toolkit.Register("ListTables", "Use this function to get a list of table names in the database.", s, s.ListTables, NoParams{})
	}
	if s.enableDescribe {
		s.Toolkit.Register("DescribeTable", "Use this function to describe a table.", s, s.DescribeTable, DescribeTableParams{})
	}
	if s.enableRun {
		s.Toolkit.Register("RunSQLQuery", "Use this function to run a SQL query and return the result (JSON rows). The result may be empty if the query does not return any data.", s, s.RunSQLQuery, RunSQLQueryParams{})
	}
	return s
}

// NewSQLToolsFromDSN opens driverName/dsn (driver must be imported by the caller).
func NewSQLToolsFromDSN(driverName, dsn string, opts ...Option) (*SQLTools, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("could not build the database connection: %w", err)
	}
	return NewSQLTools(db, opts...), nil
}

// DB returns the underlying connection.
func (s *SQLTools) DB() *sql.DB { return s.db }

// Close closes the underlying connection.
func (s *SQLTools) Close() error { return s.db.Close() }

// DetectDialect guesses the dialect from the driver type name.
func DetectDialect(db *sql.DB) string {
	name := strings.ToLower(fmt.Sprintf("%T", db.Driver()))
	switch {
	case strings.Contains(name, "sqlite"):
		return DialectSQLite
	case strings.Contains(name, "mysql"):
		return DialectMySQL
	default:
		return DialectPostgres
	}
}

func (s *SQLTools) ph(n int) string {
	if s.dialect == DialectMySQL {
		return "?"
	}
	return fmt.Sprintf("$%d", n)
}

// ListTables returns the table names as a JSON array.
func (s *SQLTools) ListTables(NoParams) (interface{}, error) {
	if s.tables != nil {
		b, _ := json.Marshal(s.tables)
		return string(b), nil
	}
	var rows *sql.Rows
	var err error
	switch s.dialect {
	case DialectSQLite:
		rows, err = s.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	case DialectMySQL:
		if s.schema != "" {
			rows, err = s.db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = ? ORDER BY table_name", s.schema)
		} else {
			rows, err = s.db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name")
		}
	default:
		schema := s.schema
		if schema == "" {
			schema = "public"
		}
		rows, err = s.db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name", schema)
	}
	if err != nil {
		return fmt.Sprintf("Error getting tables: %v", err), nil
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return fmt.Sprintf("Error getting tables: %v", err), nil
		}
		names = append(names, n)
	}
	b, _ := json.Marshal(names)
	return string(b), nil
}

type column struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Nullable bool        `json:"nullable"`
	Default  interface{} `json:"default"`
}

// DescribeTable returns the table's columns as JSON.
func (s *SQLTools) DescribeTable(p DescribeTableParams) (interface{}, error) {
	cols := []column{}
	if s.dialect == DialectSQLite {
		rows, err := s.db.Query("SELECT name, type, \"notnull\", dflt_value FROM pragma_table_info(?)", p.TableName)
		if err != nil {
			return fmt.Sprintf("Error getting table schema: %v", err), nil
		}
		defer rows.Close()
		for rows.Next() {
			var c column
			var notNull int
			var def sql.NullString
			if err := rows.Scan(&c.Name, &c.Type, &notNull, &def); err != nil {
				return fmt.Sprintf("Error getting table schema: %v", err), nil
			}
			c.Nullable = notNull == 0
			if def.Valid {
				c.Default = def.String
			}
			cols = append(cols, c)
		}
	} else {
		q := "SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name = " + s.ph(1)
		args := []interface{}{p.TableName}
		if s.schema != "" {
			q += " AND table_schema = " + s.ph(2)
			args = append(args, s.schema)
		}
		q += " ORDER BY ordinal_position"
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return fmt.Sprintf("Error getting table schema: %v", err), nil
		}
		defer rows.Close()
		for rows.Next() {
			var c column
			var nullable string
			var def sql.NullString
			if err := rows.Scan(&c.Name, &c.Type, &nullable, &def); err != nil {
				return fmt.Sprintf("Error getting table schema: %v", err), nil
			}
			c.Nullable = strings.EqualFold(nullable, "YES")
			if def.Valid {
				c.Default = def.String
			}
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		return fmt.Sprintf("Error getting table schema: table %q not found", p.TableName), nil
	}
	b, _ := json.Marshal(cols)
	return string(b), nil
}

// RunSQLQuery runs a query and returns up to limit rows (default 10) as JSON.
func (s *SQLTools) RunSQLQuery(p RunSQLQueryParams) (interface{}, error) {
	limit := 10
	if p.Limit != nil {
		limit = *p.Limit
	}
	rows, err := s.RunSQL(p.Query, limit)
	if err != nil {
		return fmt.Sprintf("Error running query: %v", err), nil
	}
	b, _ := json.Marshal(rows)
	return string(b), nil
}

// RunSQL runs sql inside a transaction and returns rows as maps. limit < 0
// returns all rows, 0 returns none.
func (s *SQLTools) RunSQL(query string, limit int) ([]map[string]interface{}, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(query)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	out, err := ScanRows(rows, limit)
	rows.Close()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return out, tx.Commit()
}

// ScanRows converts rows to maps (limit < 0 = all).
func ScanRows(rows *sql.Rows, limit int) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]interface{}{}
	for (limit < 0 || len(out) < limit) && rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]interface{}, len(cols))
		for i, c := range cols {
			m[c] = Normalize(vals[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Normalize converts driver values into JSON-friendly values.
func Normalize(v interface{}) interface{} {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339Nano)
	default:
		return v
	}
}

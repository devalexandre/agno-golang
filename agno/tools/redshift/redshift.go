// Package redshift ports Agno's RedshiftTools using the PostgreSQL wire
// protocol (github.com/lib/pq), which Amazon Redshift speaks.
//
// Deviation from Python: IAM authentication (redshift_connector's iam=True)
// is not built in; obtain temporary credentials (e.g. via
// redshift:GetClusterCredentials) and pass them as user/password, or supply
// your own *sql.DB with WithDB.
package redshift

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	_ "github.com/lib/pq" // registers the "postgres" driver

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// RedshiftTools runs queries and inspects tables in Amazon Redshift.
type RedshiftTools struct {
	toolkit.Toolkit
	host, database, user, password string
	port                           int
	ssl                            bool
	tableSchema                    string

	mu sync.Mutex
	db *sql.DB
}

// Option configures RedshiftTools.
type Option func(*RedshiftTools)

// WithHost sets the cluster endpoint (default REDSHIFT_HOST).
func WithHost(h string) Option { return func(r *RedshiftTools) { r.host = h } }

// WithPort sets the port (default 5439).
func WithPort(p int) Option { return func(r *RedshiftTools) { r.port = p } }

// WithDatabase sets the database (default REDSHIFT_DATABASE).
func WithDatabase(d string) Option { return func(r *RedshiftTools) { r.database = d } }

// WithCredentials sets user and password (default REDSHIFT_USER / REDSHIFT_PASSWORD).
func WithCredentials(user, password string) Option {
	return func(r *RedshiftTools) { r.user, r.password = user, password }
}

// WithSSL toggles SSL (default true -> sslmode=require).
func WithSSL(b bool) Option { return func(r *RedshiftTools) { r.ssl = b } }

// WithTableSchema sets the schema for table operations (default "public").
func WithTableSchema(s string) Option { return func(r *RedshiftTools) { r.tableSchema = s } }

// WithDB supplies an existing connection (bypasses DSN construction).
func WithDB(db *sql.DB) Option { return func(r *RedshiftTools) { r.db = db } }

// NoParams is used by parameterless methods.
type NoParams struct{}

// TableParams has a table name.
type TableParams struct {
	Table string `json:"table" description:"The name of the table." required:"true"`
}

// QueryParams has a SQL query.
type QueryParams struct {
	Query string `json:"query" description:"The SQL query." required:"true"`
}

// ExportParams for ExportTableToPath.
type ExportParams struct {
	Table string `json:"table" description:"The name of the table to export." required:"true"`
	Path  string `json:"path" description:"The local file path to save the file." required:"true"`
}

// NewRedshiftTools creates the toolkit. The connection is opened lazily.
func NewRedshiftTools(opts ...Option) *RedshiftTools {
	r := &RedshiftTools{
		host: os.Getenv("REDSHIFT_HOST"), database: os.Getenv("REDSHIFT_DATABASE"),
		user: os.Getenv("REDSHIFT_USER"), password: os.Getenv("REDSHIFT_PASSWORD"),
		port: 5439, ssl: true, tableSchema: "public",
	}
	r.Toolkit = toolkit.NewToolkit()
	r.Toolkit.Name = "redshift_tools"
	r.Toolkit.Description = "Inspect tables and run SQL queries against Amazon Redshift."
	for _, o := range opts {
		o(r)
	}
	r.Toolkit.Register("ShowTables", "Lists all tables in the configured schema.", r, r.ShowTables, NoParams{})
	r.Toolkit.Register("DescribeTable", "Provides the schema (column name, data type, is nullable) for a given table.", r, r.DescribeTable, TableParams{})
	r.Toolkit.Register("SummarizeTable", "Computes and returns key summary statistics for a table's columns.", r, r.SummarizeTable, TableParams{})
	r.Toolkit.Register("InspectQuery", "Shows the execution plan for a SQL query (using EXPLAIN).", r, r.InspectQuery, QueryParams{})
	r.Toolkit.Register("RunQuery", "Runs a read-only SQL query and returns the result.", r, r.RunQuery, QueryParams{})
	r.Toolkit.Register("ExportTableToPath", "Exports a table's data to a local CSV file.", r, r.ExportTableToPath, ExportParams{})
	return r
}

// DSN builds the lib/pq connection string.
func (r *RedshiftTools) DSN() string {
	u := url.URL{Scheme: "postgres", Host: r.host + ":" + strconv.Itoa(r.port), Path: "/" + r.database}
	if r.user != "" {
		u.User = url.UserPassword(r.user, r.password)
	}
	mode := "disable"
	if r.ssl {
		mode = "require"
	}
	u.RawQuery = url.Values{"sslmode": {mode}}.Encode()
	return u.String()
}

// Connect opens (or reuses) the connection.
func (r *RedshiftTools) Connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.db != nil {
		return nil
	}
	db, err := sql.Open("postgres", r.DSN())
	if err != nil {
		return err
	}
	r.db = db
	return nil
}

// Close closes the connection.
func (r *RedshiftTools) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.db == nil {
		return nil
	}
	err := r.db.Close()
	r.db = nil
	return err
}

// IsConnected reports whether a connection is open.
func (r *RedshiftTools) IsConnected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.db != nil
}

func (r *RedshiftTools) conn() (*sql.DB, error) {
	if err := r.Connect(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.db, nil
}

func str(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

func scanAll(rows *sql.Rows) ([]string, [][]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var out [][]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		out = append(out, vals)
	}
	return cols, out, rows.Err()
}

func (r *RedshiftTools) execute(query string, args ...interface{}) string {
	db, err := r.conn()
	if err != nil {
		return fmt.Sprintf("Error executing query: %v", err)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Sprintf("Error executing query: %v", err)
	}
	defer rows.Close()
	cols, data, err := scanAll(rows)
	if err != nil {
		return fmt.Sprintf("Error executing query: %v", err)
	}
	if len(cols) == 0 {
		return "Query executed successfully."
	}
	if len(data) == 0 {
		return fmt.Sprintf("Query returned no results.\nColumns: %s", strings.Join(cols, ", "))
	}
	lines := []string{strings.Join(cols, ",")}
	for _, row := range data {
		parts := make([]string, len(row))
		for i, v := range row {
			parts[i] = str(v)
		}
		lines = append(lines, strings.Join(parts, ","))
	}
	return strings.Join(lines, "\n")
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func (r *RedshiftTools) fullTable(table string) string {
	return quoteIdent(r.tableSchema) + "." + quoteIdent(table)
}

// ShowTables lists tables in the schema.
func (r *RedshiftTools) ShowTables(NoParams) (interface{}, error) {
	return r.execute("SELECT table_name FROM information_schema.tables WHERE table_schema = $1;", r.tableSchema), nil
}

// DescribeTable lists column name, type and nullability.
func (r *RedshiftTools) DescribeTable(p TableParams) (interface{}, error) {
	return r.execute(`SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2;`, r.tableSchema, p.Table), nil
}

var numericTypes = []string{"integer", "int", "bigint", "smallint", "numeric", "decimal", "real", "double precision", "float"}
var textTypes = []string{"char", "varchar", "text", "uuid"}

func containsAny(s string, subs []string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

// SummarizeTable computes per-column statistics.
func (r *RedshiftTools) SummarizeTable(p TableParams) (interface{}, error) {
	db, err := r.conn()
	if err != nil {
		return fmt.Sprintf("Error summarizing table: %v", err), nil
	}
	rows, err := db.Query(`SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2;`, r.tableSchema, p.Table)
	if err != nil {
		return fmt.Sprintf("Error summarizing table: %v", err), nil
	}
	_, columns, err := scanAll(rows)
	rows.Close()
	if err != nil {
		return fmt.Sprintf("Error summarizing table: %v", err), nil
	}
	if len(columns) == 0 {
		return fmt.Sprintf("Error: Table '%s' not found in schema '%s'.", p.Table, r.tableSchema), nil
	}
	parts := []string{fmt.Sprintf("Summary for table: %s\n", p.Table)}
	full := r.fullTable(p.Table)
	for _, c := range columns {
		name, dtype := str(c[0]), str(c[1])
		col := quoteIdent(name)
		lower := strings.ToLower(dtype)
		var q string
		switch {
		case containsAny(lower, numericTypes):
			q = fmt.Sprintf(`SELECT COUNT(*) AS total_rows, COUNT(%[1]s) AS non_null_rows, MIN(%[1]s) AS min, MAX(%[1]s) AS max, AVG(%[1]s) AS average, STDDEV(%[1]s) AS std_deviation FROM %[2]s;`, col, full)
		case containsAny(lower, textTypes):
			q = fmt.Sprintf(`SELECT COUNT(*) AS total_rows, COUNT(%[1]s) AS non_null_rows, COUNT(DISTINCT %[1]s) AS unique_values, AVG(LEN(%[1]s)) AS avg_length FROM %[2]s;`, col, full)
		default:
			continue
		}
		srows, err := db.Query(q)
		if err != nil {
			return fmt.Sprintf("Error summarizing table: %v", err), nil
		}
		names, stats, err := scanAll(srows)
		srows.Close()
		if err != nil {
			return fmt.Sprintf("Error summarizing table: %v", err), nil
		}
		parts = append(parts, fmt.Sprintf("\n--- Column: %s (Type: %s) ---", name, dtype))
		if len(stats) == 0 {
			parts = append(parts, "  No statistics available")
			continue
		}
		for i, k := range names {
			v := stats[0][i]
			if f, ok := v.(float64); ok {
				parts = append(parts, fmt.Sprintf("  %s: %.2f", k, f))
			} else {
				parts = append(parts, fmt.Sprintf("  %s: %s", k, str(v)))
			}
		}
	}
	return strings.Join(parts, "\n"), nil
}

// InspectQuery returns the EXPLAIN plan.
func (r *RedshiftTools) InspectQuery(p QueryParams) (interface{}, error) {
	return r.execute("EXPLAIN " + p.Query), nil
}

// RunQuery runs a SQL query and returns CSV-like output.
func (r *RedshiftTools) RunQuery(p QueryParams) (interface{}, error) {
	return r.execute(p.Query), nil
}

// ExportTableToPath writes the table to a CSV file.
func (r *RedshiftTools) ExportTableToPath(p ExportParams) (interface{}, error) {
	db, err := r.conn()
	if err != nil {
		return fmt.Sprintf("Error exporting table: %v", err), nil
	}
	rows, err := db.Query("SELECT * FROM " + r.fullTable(p.Table) + ";")
	if err != nil {
		return fmt.Sprintf("Error exporting table: %v", err), nil
	}
	defer rows.Close()
	cols, data, err := scanAll(rows)
	if err != nil {
		return fmt.Sprintf("Error exporting table: %v", err), nil
	}
	f, err := os.Create(p.Path)
	if err != nil {
		return fmt.Sprintf("Error exporting table: %v", err), nil
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write(cols)
	for _, row := range data {
		rec := make([]string, len(row))
		for i, v := range row {
			if v == nil {
				rec[i] = ""
			} else {
				rec[i] = str(v)
			}
		}
		_ = w.Write(rec)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Sprintf("Error exporting table: %v", err), nil
	}
	return fmt.Sprintf("Successfully exported table '%s' to '%s'.", p.Table, p.Path), nil
}

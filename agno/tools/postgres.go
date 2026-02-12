package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresTool provides PostgreSQL database operations.
// Implements ConnectableTool for lifecycle management.
type PostgresTool struct {
	toolkit.Toolkit
	connStr string
	db      *sql.DB
}

type PostgresQueryParams struct {
	SQL string `json:"sql" description:"The SQL query to execute." required:"true"`
}

type PostgresListTablesParams struct {
	Schema string `json:"schema,omitempty" description:"Database schema. Default: public."`
}

type PostgresDescribeTableParams struct {
	TableName string `json:"table_name" description:"The table name to describe." required:"true"`
	Schema    string `json:"schema,omitempty" description:"Database schema. Default: public."`
}

// NewPostgresTool creates a new PostgreSQL tool.
// If connStr is empty, it reads from the DATABASE_URL environment variable.
func NewPostgresTool(connStr string) *PostgresTool {
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}

	t := &PostgresTool{
		connStr: connStr,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "PostgresTool"
	tk.Description = "PostgreSQL database operations: query, list tables, and describe table schemas."

	t.Toolkit = tk
	t.Toolkit.Register("Query", "Execute a read-only SQL query and return results.", t, t.Query, PostgresQueryParams{})
	t.Toolkit.Register("ListTables", "List all tables in the database.", t, t.ListTables, PostgresListTablesParams{})
	t.Toolkit.Register("DescribeTable", "Describe a table's columns and types.", t, t.DescribeTable, PostgresDescribeTableParams{})

	return t
}

// Connect opens the database connection.
func (t *PostgresTool) Connect() error {
	if t.connStr == "" {
		return fmt.Errorf("DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", t.connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping postgres: %w", err)
	}
	t.db = db
	return nil
}

// Close closes the database connection.
func (t *PostgresTool) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *PostgresTool) ensureConnected() error {
	if t.db == nil {
		return t.Connect()
	}
	return nil
}

// Query executes a SQL query and returns results as JSON.
func (t *PostgresTool) Query(params PostgresQueryParams) (interface{}, error) {
	if err := t.ensureConnected(); err != nil {
		return nil, err
	}

	rows, err := t.db.Query(params.SQL)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	output, _ := json.Marshal(map[string]interface{}{
		"columns": columns,
		"rows":    results,
		"count":   len(results),
	})

	return string(output), nil
}

// ListTables lists all tables in the specified schema.
func (t *PostgresTool) ListTables(params PostgresListTablesParams) (interface{}, error) {
	schema := params.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = '%s'
		ORDER BY table_name`, schema)

	return t.Query(PostgresQueryParams{SQL: query})
}

// DescribeTable describes the columns of a table.
func (t *PostgresTool) DescribeTable(params PostgresDescribeTableParams) (interface{}, error) {
	schema := params.Schema
	if schema == "" {
		schema = "public"
	}

	query := fmt.Sprintf(`
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = '%s' AND table_name = '%s'
		ORDER BY ordinal_position`,
		schema, strings.ReplaceAll(params.TableName, "'", "''"))

	return t.Query(PostgresQueryParams{SQL: query})
}

// Execute implements the toolkit.Tool interface.
func (t *PostgresTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}

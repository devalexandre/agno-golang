package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DuckDBTool provides DuckDB analytics SQL operations.
// DuckDB is an in-process analytical database - users must import a DuckDB driver
// (e.g., github.com/marcboeker/go-duckdb) and register it as "duckdb" before use.
// If no driver is available, the tool can work with a pre-opened *sql.DB.
type DuckDBTool struct {
	toolkit.Toolkit
	dbPath string
	db     *sql.DB
}

type DuckDBQueryParams struct {
	SQL string `json:"sql" description:"The SQL query to execute." required:"true"`
}

type DuckDBLoadCSVParams struct {
	Path      string `json:"path" description:"File path to the CSV file." required:"true"`
	TableName string `json:"table_name" description:"Name of the table to create." required:"true"`
}

type DuckDBExportCSVParams struct {
	SQL  string `json:"sql" description:"SQL query whose results will be exported." required:"true"`
	Path string `json:"path" description:"Output CSV file path." required:"true"`
}

// NewDuckDBTool creates a new DuckDB tool.
// dbPath is the path to the DuckDB database file. Use ":memory:" for in-memory.
// If dbPath is empty, it reads from DUCKDB_PATH or defaults to ":memory:".
func NewDuckDBTool(dbPath string) *DuckDBTool {
	if dbPath == "" {
		dbPath = os.Getenv("DUCKDB_PATH")
	}
	if dbPath == "" {
		dbPath = ":memory:"
	}

	t := &DuckDBTool{
		dbPath: dbPath,
	}

	tk := toolkit.NewToolkit()
	tk.Name = "DuckDBTool"
	tk.Description = "DuckDB analytical SQL engine: query data, load CSV/Parquet files, and export results."

	t.Toolkit = tk
	t.Toolkit.Register("Query", "Execute a SQL query on DuckDB.", t, t.Query, DuckDBQueryParams{})
	t.Toolkit.Register("LoadCSV", "Load a CSV file into a DuckDB table.", t, t.LoadCSV, DuckDBLoadCSVParams{})
	t.Toolkit.Register("ExportCSV", "Export query results to a CSV file.", t, t.ExportCSV, DuckDBExportCSVParams{})

	return t
}

// NewDuckDBToolWithDB creates a DuckDB tool from an existing database connection.
func NewDuckDBToolWithDB(db *sql.DB) *DuckDBTool {
	t := &DuckDBTool{db: db}

	tk := toolkit.NewToolkit()
	tk.Name = "DuckDBTool"
	tk.Description = "DuckDB analytical SQL engine: query data, load CSV/Parquet files, and export results."

	t.Toolkit = tk
	t.Toolkit.Register("Query", "Execute a SQL query on DuckDB.", t, t.Query, DuckDBQueryParams{})
	t.Toolkit.Register("LoadCSV", "Load a CSV file into a DuckDB table.", t, t.LoadCSV, DuckDBLoadCSVParams{})
	t.Toolkit.Register("ExportCSV", "Export query results to a CSV file.", t, t.ExportCSV, DuckDBExportCSVParams{})

	return t
}

// Connect opens the DuckDB database.
func (t *DuckDBTool) Connect() error {
	if t.db != nil {
		return nil
	}
	db, err := sql.Open("duckdb", t.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open duckdb: %w", err)
	}
	t.db = db
	return nil
}

// Close closes the DuckDB database.
func (t *DuckDBTool) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

func (t *DuckDBTool) ensureConnected() error {
	if t.db == nil {
		return t.Connect()
	}
	return nil
}

// Query executes a SQL query and returns results.
func (t *DuckDBTool) Query(params DuckDBQueryParams) (interface{}, error) {
	if err := t.ensureConnected(); err != nil {
		return nil, err
	}

	rows, err := t.db.Query(params.SQL)
	if err != nil {
		return nil, fmt.Errorf("duckdb query failed: %w", err)
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

// LoadCSV loads a CSV file into a DuckDB table.
func (t *DuckDBTool) LoadCSV(params DuckDBLoadCSVParams) (interface{}, error) {
	if err := t.ensureConnected(); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s AS SELECT * FROM read_csv_auto('%s')",
		params.TableName, params.Path)

	_, err := t.db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("failed to load CSV: %w", err)
	}

	return map[string]interface{}{
		"status":     "ok",
		"table_name": params.TableName,
		"source":     params.Path,
	}, nil
}

// ExportCSV exports query results to a CSV file.
func (t *DuckDBTool) ExportCSV(params DuckDBExportCSVParams) (interface{}, error) {
	if err := t.ensureConnected(); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("COPY (%s) TO '%s' (HEADER, DELIMITER ',')", params.SQL, params.Path)

	_, err := t.db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("failed to export CSV: %w", err)
	}

	return map[string]interface{}{
		"status": "ok",
		"path":   params.Path,
	}, nil
}

// Execute implements the toolkit.Tool interface.
func (t *DuckDBTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
